package mpv

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSubtitleGuardRejectsStaleFilesReplaysAndExistingTracks(t *testing.T) {
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("lua unavailable")
	}
	dir := t.TempDir()
	script := filepath.Join("..", "..", "modernz", "subtitles.lua")
	offsetInput := filepath.Join(dir, "subtitle_offset_input.lua")
	if err := os.WriteFile(offsetInput, []byte("return require 'mp.input'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	test := `
local events, messages, state = {}, {}, {['playlist-pos']=0, ['playlist/0/id']=1, ['track-list']={}}
local observed = {}
local calls = 0
local osd_text, osd_duration, fail_add = nil, nil, false
local last_add, timers = nil, {}
local prompt, last_save, input_count, input_errors = nil, nil, 0, 0
package.preload['mp.input'] = function() return {
 get=function(options) prompt=options; input_count=input_count+1 end,
 select=function(options) prompt=options end,
 terminate=function() if prompt then prompt.closed() end end,
 log_error=function() input_errors=input_errors+1 end,
} end
mp = {
 command_native=function(args) assert(args[1]=='expand-path'); return arg[2] end,
 set_property_native=function(key,value) state[key]=value end,
 set_property_number=function(key,value) state[key]=value end,
 get_property_native=function(key,default) if state[key]~=nil then return state[key] end; return default end,
 get_property_number=function(key,default) return state[key] or default end,
 observe_property=function(name,_,fn) observed[name]=fn end,
 register_event=function(name,fn) events[name]=fn end,
 register_script_message=function(name,fn) messages[name]=fn end,
 add_timeout=function(_,fn) timers[#timers+1]=fn end,
 commandv=function(...)
   local args={...}
   if args[1]=='sub-add' then
     if args[2]=='fresh.srt' then state['track-list']={{type='sub',id=1,['external-filename']='fresh.srt'}}; state['sid']=1 end
     calls=calls+1; last_add=args; return not fail_add else last_save=args end
 end,
 osd_message=function(text,duration) osd_text,osd_duration=text,duration end,
}
dofile(arg[1])
local add=messages['javboss-subtitle-add']
events['start-file'](); events['file-loaded']()
add('1','1','one.srt','source','zh-CN','30'); assert(calls==1); assert(state['sub-delay']==30)
state['track-list']={{type='sub', id=1, ['external-filename']='one.srt'}}
add('1','1','one.srt','source','zh-CN','30'); assert(calls==1)
add('1','1','','','','12.5'); assert(calls==1); assert(state['sub-delay']==12.5)
state['track-list']={}
events['end-file']()
add('1','1','one.srt','source','zh-CN','30'); assert(calls==1)
assert(state['sub-delay']==12.5)
-- Replaying the very same entry still invalidates the old generation.
events['start-file'](); events['file-loaded']()
add('1','1','one.srt','source','zh-CN','30'); assert(calls==1)
assert(state['sub-delay']==12.5)
add('2','1','one.srt','source','zh-CN','30'); assert(calls==2)
state['playlist/0/id']=2
add('2','1','one.srt','source','zh-CN','90'); assert(calls==2); assert(state['sub-delay']==30)
add('2','2','','','','0'); assert(calls==2); assert(state['sub-delay']==0)
state['user-data/javboss/subtitle-editable-entries']={['2']=true}
local open=messages['javboss-subtitle-offset']
local saved=messages['javboss-subtitle-saved']
open(); assert(input_count==1)
prompt.submit('-3601'); prompt.submit('3601'); prompt.submit('nan'); prompt.submit('')
assert(input_errors==4); assert(state['sub-delay']==0)
prompt.submit('12.3456'); assert(state['sub-delay']==12.346)
assert(last_save[2]=='javboss-subtitle-save' and last_save[3]=='2' and last_save[4]=='2' and last_save[5]=='1' and last_save[6]=='12346')
open(); assert(input_count==1) -- wait for the in-flight save
add('2','2','one.srt','source','zh-CN','30'); assert(state['sub-delay']==12.346)
saved('1','2','1','saved'); open(); assert(input_count==1) -- stale acknowledgement
saved('2','2','1','error'); open(); assert(input_count==2) -- retry after failure
prompt.submit('0'); assert(state['sub-delay']==0)
saved('2','2','2','saved'); open(); assert(input_count==3)
prompt.submit('-12.3456'); assert(state['sub-delay']==-12.346)
assert(last_save[6]=='-12346')
saved('2','2','3','saved'); open(); assert(prompt.default_text=='-12.346')
local old_prompt=prompt
events['end-file'](); events['start-file'](); events['file-loaded']()
add('3','2','','','','45'); assert(state['sub-delay']==45)
old_prompt.submit('99'); assert(state['sub-delay']==45) -- input belongs to an old playback
add('3','2','','','','-1.5'); assert(state['sub-delay']==-1.5)
local function flush()
 local pending=timers; timers={}
 for _, fn in ipairs(pending) do fn() end
end
local menu=messages['javboss-subtitle-menu']
menu()
assert(#prompt.items==4 and prompt.items[1]=='选择字幕' and prompt.items[2]=='关闭字幕' and prompt.items[3]=='设置偏移' and prompt.items[4]=='查找字幕')
prompt.submit(2); flush()
assert(state['sid']==false and state['secondary-sid']==false)
add('3','2','late.srt','source','zh-CN','0')
assert(last_add[3]=='auto' and state['sid']==false) -- late downloads stay disabled
menu(); prompt.submit(1); flush()
assert(last_save[1]=='script-binding' and last_save[2]=='select/select-sid')
menu(); prompt.submit(3); flush()
assert(prompt.items==nil and prompt.default_text=='0.000')
prompt.submit('-2.5'); assert(state['sub-delay']==-2.5 and last_save[6]=='-2500')
saved('3','2','1','saved')
menu(); prompt.closed(); flush() -- Escape only dismisses the menu
assert(state['sub-delay']==-2.5)
menu()
local stale_menu=prompt
events['end-file'](); events['start-file'](); events['file-loaded']()
state['sid']=42
stale_menu.submit(2); flush(); assert(state['sid']==42)
menu(); prompt.submit(2)
events['end-file'](); events['start-file'](); events['file-loaded']()
flush(); assert(state['sid']==42) -- a queued action cannot affect the next video
messages['javboss-subtitle-action']('2', '3', '2')
assert(state['sid']==42) -- an anchored popup from an old playback is ignored
messages['javboss-subtitle-action']('2', '5', '999')
assert(state['sid']==42) -- another playlist entry cannot close this video's subtitles
local status=messages['javboss-subtitle-status']
local generation=tostring(state['user-data/javboss/subtitle-session'].generation)
for key,text in pairs({searching='正在查找字幕…',ready='字幕已加载',existing='检测到已有字幕',not_found='未找到中文字幕',ambiguous='无法自动匹配字幕',error='字幕下载失败'}) do
 status(generation,'2',key)
 assert(osd_text==text and osd_duration==(key=='searching' and 50 or 3))
end
status(generation,'2','searching')
status('1','2','error'); status(generation,'999','error'); status(generation,'2','unknown')
assert(osd_text=='正在查找字幕…')
status(generation,'2','disabled'); assert(osd_text=='' and osd_duration==0)
state['track-list']={}; fail_add=true
add(generation,'2','missing.srt','source','zh-CN','0'); assert(osd_text=='字幕下载失败')
fail_add=false
add(generation,'2','one.srt','source','zh-CN','0'); assert(osd_text=='字幕已加载')
state['track-list']={{type='sub', id=1, ['external-filename']='one.srt'}}
add(generation,'2','one.srt','source','zh-CN','0'); assert(osd_text=='检测到已有字幕')
-- A different downloaded path becomes a separate selectable track.
local before_add=calls
add(generation,'2','download.srt','subtitlecat','zh-CN','0','ready','true','','download:zh-CN')
assert(calls==before_add+1 and last_add[3]=='select')
state['track-list']={{type='sub',id=1,['external-filename']='one.srt'}, {type='sub',id=2,['external-filename']='download.srt'}}
state['sid']=1
add(generation,'2','download.srt','subtitlecat','zh-CN','0','ready','true','','download:zh-CN')
assert(calls==before_add+1 and state['sid']==2) -- reselect, never duplicate
assert(state['user-data/javboss/subtitle-downloaded']==true)
menu(); assert(#prompt.items==3); prompt.closed(); flush()
state['sid']=false; observed.sid()
menu(); assert(#prompt.items==3); prompt.closed(); flush() -- hidden even when disabled
state['track-list']={{type='sub',id=1,['external-filename']='one.srt'}}
observed['track-list']()
assert(state['user-data/javboss/subtitle-downloaded']==false)
menu(); assert(#prompt.items==4 and prompt.items[4]=='查找字幕'); prompt.closed(); flush()

status(generation,'2','searching'); events['end-file']()
assert(osd_text=='' and osd_duration==0)
status(generation,'2','error'); assert(osd_text=='')
-- Restore off for existing/cache tracks without recording initialization.
events['start-file'](); events['file-loaded']()
generation=tostring(state['user-data/javboss/subtitle-session'].generation)
state['track-list']={{type='sub',id=1}}; state['sid']=1
add(generation,'2','','','','0','','false','off')
assert(state['sid']==false)
local before=last_save
observed.sid(); assert(last_save==before)
state['sid']=1; observed.sid()
assert(last_save[2]=='javboss-subtitle-selection' and last_save[5]=='embedded:1')
state['sid']=false; observed.sid()
assert(last_save[5]=='off')
-- A new download explicitly opens and remembers on despite a previous off.
state['track-list']={}
add(generation,'2','fresh.srt','source','zh-CN','0','ready','true','off','download:zh-CN')
assert(last_add[3]=='select' and last_save[2]=='javboss-subtitle-selection' and last_save[5]=='download:zh-CN')

`
	path := filepath.Join(dir, "test.lua")
	if err := os.WriteFile(path, []byte(test), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(lua, path, script, offsetInput).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestSubtitleWaitCancelledOnFileChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	e := &playlistEvents{subtitleCancel: cancel}
	e.handle(playlistEvent{Event: "start-file", EntryID: 2})
	if ctx.Err() != context.Canceled {
		t.Fatal("previous subtitle wait still active")
	}
	ctx, cancel = context.WithCancel(context.Background())
	e.subtitleCancel = cancel
	e.handle(playlistEvent{Event: "end-file"})
	if ctx.Err() != context.Canceled {
		t.Fatal("ended subtitle wait still active")
	}
}

func TestSubtitleSavesRetainVideoAndInputOrder(t *testing.T) {
	previousDial := dialMPVIPCOverride
	dialMPVIPCOverride = func(string, time.Duration) (io.ReadWriteCloser, error) { return nil, os.ErrNotExist }
	defer func() { dialMPVIPCOverride = previousDial }()
	type write struct{ entry, duration int64 }
	writes := make(chan write, 2)
	e := &playlistEvents{entryID: 7, loaded: true, subtitleSavers: map[int64]SubtitleOffsetSaver{
		7: func(_ context.Context, ms int64) error { writes <- write{7, ms}; return nil },
		8: func(_ context.Context, ms int64) error { writes <- write{8, ms}; return nil },
	}}
	for _, args := range [][]string{
		nil, {"other", "1", "7", "1", "0"}, {"javboss-subtitle-save", "1", "9", "1", "0"},
		{"javboss-subtitle-save", "1", "7", "1", "-3600001"}, {"javboss-subtitle-save", "1", "7", "1", "3600001"},
		{"javboss-subtitle-save", "1", "7", "1", "1.5"}, {"javboss-subtitle-save", "bad", "7", "1", "0"},
	} {
		if e.handle(playlistEvent{Event: "client-message", Args: args}) != nil {
			t.Fatalf("accepted invalid save %v", args)
		}
	}
	first := e.handle(playlistEvent{Event: "client-message", Args: []string{"javboss-subtitle-save", "1", "7", "1", "12346"}})
	e.handle(playlistEvent{Event: "start-file", EntryID: 8})
	e.handle(playlistEvent{Event: "file-loaded"})
	second := e.handle(playlistEvent{Event: "client-message", Args: []string{"javboss-subtitle-save", "2", "8", "1", "-30000"}})
	if first == nil || second == nil {
		t.Fatal("save callback missing")
	}
	// Start the second callback first, as Go's event goroutines can be scheduled
	// out of order. It must wait for the first accepted save.
	done := make(chan struct{})
	go func() { second(); close(done) }()
	first()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queued save blocked")
	}
	if got := <-writes; got != (write{7, 12346}) {
		t.Fatalf("first write = %+v", got)
	}
	if got := <-writes; got != (write{8, -30000}) {
		t.Fatalf("second write = %+v", got)
	}
}
