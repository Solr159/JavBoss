package mpv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSubtitleOffsetDialogLayoutAndEditing(t *testing.T) {
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("lua unavailable")
	}
	dir := t.TempDir()
	script := filepath.Join("..", "..", "modernz", "subtitle_offset_input.lua")
	// Capture drawing coordinates and key handlers; exercise the actual dialog
	// through resize, keyboard submission and cleanup.
	test := `
local w,h=960,540
local mx,my=0,0
local keys,names,state,rects,texts={},{},{},{},{}
local resize,removed,closed,saved=nil,false,0,nil
package.preload['mp.assdraw']=function() return {ass_new=function()
 rects,texts={},{}
 return {text='',new_event=function() end,pos=function() end,an=function() end,
 append=function(_,s) texts[#texts+1]=s end,draw_start=function() end,draw_stop=function() end,
 round_rect_cw=function(_,x1,y1,x2,y2) rects[#rects+1]={x1,y1,x2,y2} end}
end} end
mp={
 get_osd_size=function() return w,h end,
 get_mouse_pos=function() return mx,my end,
 get_property_native=function(_,fallback) return fallback end,
 get_property_number=function(_,fallback) return fallback end,
 command_native=function(args) return args[2] end,
 create_osd_overlay=function() return {update=function() removed=false end,remove=function() removed=true end} end,
 set_property_native=function(k,v) state[k]=v end,
 get_property=function() return '-12.5' end,
 observe_property=function(_,_,fn) resize=fn end,
 add_forced_key_binding=function(k,name,fn) keys[k]=fn; names[name]=k end,
 remove_key_binding=function(name) keys[names[name]]=nil; names[name]=nil end,
}
local input=dofile(arg[1])
local function open()
 input.get({default_text='1.000',closed=function() closed=closed+1 end,
 submit=function(value) saved=value; input.terminate() end})
end
local function key(k) assert(keys[k],k)() end
local function type_text(value)
 for ch in value:gmatch('.') do keys.any_unicode({event='press',key_text=ch}) end
end
open()
assert(table.concat(texts):find('│',1,true),'initial input cursor is missing')
assert(table.concat(texts):find('输入完成后按 Enter 保存',1,true),'save instruction is missing')
for _,size in ipairs({{960,540},{320,180},{1920,1080}}) do
 w,h=size[1],size[2]; resize()
 assert(#rects==1,'dialog should have a single panel without button or input backgrounds')
 local panel=rects[1]
 assert(math.abs(panel[1]+panel[3]-w)<0.001 and math.abs(panel[2]+panel[4]-h)<0.001,'dialog is not centered')
 assert(panel[1]>=0 and panel[2]>=0 and panel[3]<=w and panel[4]<=h,'dialog exceeds viewport')
end
-- Default value is selected; typing replaces it. Cursor editing supports decimals.
type_text('-12.5'); key('LEFT'); key('BS'); type_text('.'); key('END'); key('BS'); type_text('75')
key('ENTER'); assert(saved=='-12.75',saved)
assert(removed and closed==1 and next(keys)==nil and state['user-data/javboss/subtitle-offset-open']==false)
open(); key('Ctrl+a'); type_text('9'); key('ESC'); assert(saved=='-12.75' and closed==2)
open(); key('Ctrl+u'); type_text('q'); type_text('3601')
input.log_error('invalid range'); assert(table.concat(texts):find('invalid range',1,true))
key('Ctrl+v') -- numeric clipboard text inserts at cursor
key('Ctrl+a'); key('Ctrl+v')
mx,my=w/2,h/2
keys.MBTN_LEFT({event='up'}); assert(saved=='-12.75' and closed==2,'inside click must not save or close')
key('ENTER'); assert(saved=='-12.5' and closed==3)
open(); key('ESC'); assert(closed==4 and next(keys)==nil)
input.terminate(); assert(closed==4)
open(); key('KP_SUBTRACT'); key('KP2'); key('KP_DEC'); key('KP5'); key('KP_ENTER')
assert(saved=='-2.5' and closed==5)
-- Outside clicks cancel without saving, including after a resize.
for _,size in ipairs({{960,540},{320,180},{1920,1080}}) do
 open(); type_text('99'); w,h=size[1],size[2]; resize()
 local panel=rects[1]
 mx,my=panel[1]-1,(panel[2]+panel[4])/2
 keys.MBTN_LEFT({event='down'}); assert(next(keys)~=nil,'must consume the release before closing')
 keys.MBTN_LEFT({event='up'})
 assert(saved=='-2.5' and next(keys)==nil and removed,'outside click must cancel and clean up')
end
`
	path := filepath.Join(dir, "test.lua")
	if err := os.WriteFile(path, []byte(test), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(lua, path, script).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
