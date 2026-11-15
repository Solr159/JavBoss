-- The guard lives in MPV so checking the current file and adding a subtitle
-- happen in one script callback, rather than two racy JSON IPC round trips.
local offset_input = dofile(mp.command_native({'expand-path', '~~/scripts/subtitle_offset_input.lua'}))
local generation = 0
local loaded = false
local input = require 'mp.input'
local edited = false
local revision = 0
local saving = false
local input_open = false
local subtitles_disabled = false
local visibility_ready = false
local last_visibility
local preferred_track = ''
local last_track = ''
local track_keys = {}
local status_overlay = mp.create_osd_overlay('ass-events')
status_overlay.z = 100
local status_timer, active_status_text
local publish_menu
local status_text = {
    searching = '正在查找字幕…',
    ready = '字幕已加载',
    existing = '检测到已有字幕',
    not_found = '未找到中文字幕',
    ambiguous = '无法自动匹配字幕',
    error = '字幕下载失败',
}
local function clear_status()
    if status_timer then status_timer:kill(); status_timer = nil end
    active_status_text = nil
    status_overlay:remove()
end
local function render_status()
    if not active_status_text then return end
    local w, h = mp.get_osd_size()
    if w <= 0 or h <= 0 then return end
    local margins = mp.get_property_native('user-data/osc/margins', {})
    local bottom = math.max(72, (margins.b or 0) * 720 + 12)
    status_overlay.res_x, status_overlay.res_y = w / h * 720, 720
    -- Keep loading feedback above the bottom controls without replacing native
    -- OSD messages such as the startup shortcut hint or volume feedback.
    status_overlay.data = string.format(
        '{\\an1\\pos(18,%f)\\fs20\\bord1.5\\shad1\\q2\\1c&HFFFFFF&\\3c&H000000&}%s',
        math.max(24, 720 - bottom), active_status_text)
    status_overlay:update()
end
local function show_status(status)
    mp.set_property_native('user-data/javboss/subtitle-status', status)
    clear_status()
    active_status_text = status_text[status]
    if active_status_text then
        render_status()
        status_timer = mp.add_timeout(status == 'searching' and 50 or 3, clear_status)
    end
end
mp.observe_property('osd-dimensions', 'native', render_status)
mp.observe_property('user-data/osc/margins', 'native', render_status)
local function current_entry()
    local pos = mp.get_property_number('playlist-pos', -1)
    return mp.get_property_number('playlist/' .. pos .. '/id', -1)
end
local function publish()
    if publish_menu then publish_menu() end
    mp.set_property_native('user-data/javboss/subtitle-session', {
        generation = generation,
        entry = current_entry(),
        loaded = loaded,
    })
end
local function editable()
    local entries = mp.get_property_native('user-data/javboss/subtitle-editable-entries', {})
    return loaded and entries[tostring(current_entry())] == true
end
local function close_input()
    offset_input.terminate()
    if input_open then input.terminate() end
end
mp.register_event('start-file', function()
    close_input()
    show_status('')
    generation = generation + 1
    loaded = false
    edited, saving = false, false
    subtitles_disabled = false
    visibility_ready, last_visibility = false, nil
    preferred_track, last_track, track_keys = '', '', {}
    mp.set_property_native('user-data/javboss/subtitle-downloaded', false)
    revision = 0
    mp.set_property_native('user-data/javboss/subtitle-editable', false)
    publish()
end)
mp.register_event('file-loaded', function()
    loaded = true
    mp.set_property_native('user-data/javboss/subtitle-editable', editable())
    publish()
end)
mp.register_event('end-file', function()
    close_input()
    show_status('')
    loaded = false
    visibility_ready = false
    mp.set_property_native('user-data/javboss/subtitle-downloaded', false)
    mp.set_property_native('user-data/javboss/subtitle-editable', false)
    publish()
end)
local function has_subtitle()
    for _, track in ipairs(mp.get_property_native('track-list', {})) do
        if track.type == 'sub' then return true end
    end
    return false
end
local function subtitle_visible()
    local sid = mp.get_property_native('sid')
    return sid ~= nil and sid ~= false and sid ~= 'no' and mp.get_property_native('sub-visibility', true)
end
local function track_key(track)
    local path = track['external-filename']
    if path then return track_keys[path] or ('local:' .. (path:match('[^/\\]+$') or path)) end
    return 'embedded:' .. tostring(track.id)
end
local function has_downloaded_subtitle()
    for _, track in ipairs(mp.get_property_native('track-list', {})) do
        if track.type == 'sub' and track_key(track):match('^download:') then return true end
    end
    return false
end
local function menu_items()
    local items = {}
    if not loaded then return items end
    local sid = mp.get_property_native('sid')
    local visible = subtitle_visible()
    local tracks = mp.get_property_native('track-list', {})
    for _, track in ipairs(tracks) do
        if track.type == 'sub' then
            local key = track_key(track)
            local downloaded = key:match('^download:') ~= nil
            local path = track['external-filename']
            local filename = path and (path:match('[^/\\]+$') or path)
                or track.title or ('内嵌字幕 ' .. (track.lang or tostring(track.id)))
            local label = filename .. '（' .. (downloaded and '下载' or '本地') .. '）'
            items[#items + 1] = {name = 'track_' .. track.id, label = label, action = key, selected = visible and sid == track.id}
        end
    end
    items[#items + 1] = {name = 'off', label = '关闭字幕', action = 'off'}
    items[#items + 1] = {name = 'offset', label = '设置偏移', action = 'offset'}
    if not has_downloaded_subtitle() then
        items[#items + 1] = {name = 'find', label = '查找字幕', action = 'find'}
    end
    return items
end
publish_menu = function()
    mp.set_property_native('user-data/javboss/subtitle-menu-items', menu_items())
end
local function publish_downloaded_subtitle()
    publish_menu()
    mp.set_property_native('user-data/javboss/subtitle-downloaded', loaded and has_downloaded_subtitle())
end
mp.observe_property('track-list', 'native', publish_downloaded_subtitle)
local function selected_key()
    local sid = mp.get_property_native('sid')
    for _, track in ipairs(mp.get_property_native('track-list', {})) do
        if track.type == 'sub' and track.id == sid then return track_key(track) end
    end
    return preferred_track
end
local function snapshot_selection()
    last_visibility, last_track = subtitle_visible(), selected_key()
end
local function restore_track()
    if subtitles_disabled or preferred_track == '' then return end
    for _, track in ipairs(mp.get_property_native('track-list', {})) do
        if track.type == 'sub' and track_key(track) == preferred_track then
            mp.set_property_native('sid', track.id)
            return
        end
    end
end
local function remember_visibility(enabled)
    preferred_track = enabled and selected_key() or 'off'
    snapshot_selection()
    mp.commandv('script-message', 'javboss-subtitle-selection', tostring(generation), tostring(current_entry()), preferred_track)
end
local function apply_visibility(enabled)
    subtitles_disabled = not enabled
    mp.set_property_native('sub-visibility', true)
    if enabled then
        mp.commandv('set', 'sid', 'auto')
        restore_track()
    else mp.set_property_native('sid', false) end
    snapshot_selection()
end
local function visibility_changed()
    publish_menu()
    if not loaded or not visibility_ready or not has_subtitle() then return end
    local enabled, key = subtitle_visible(), selected_key()
    if enabled == last_visibility and (not enabled or key == last_track) then return end
    subtitles_disabled = not enabled
    remember_visibility(enabled)
end
mp.observe_property('sid', 'native', visibility_changed)
mp.observe_property('sub-visibility', 'bool', visibility_changed)
mp.register_script_message('javboss-subtitle-status', function(expected, entry, status)
    if not loaded or generation ~= tonumber(expected) or current_entry() ~= tonumber(entry) then return end
    if status_text[status] or status == 'disabled' then show_status(status) end
end)
mp.register_script_message('javboss-subtitle-add', function(expected, entry, path, title, language, delay, status, force_enable, saved_track, key)
    if not loaded or generation ~= tonumber(expected) then return end
    if current_entry() ~= tonumber(entry) then return end
    delay = tonumber(delay)
    if not delay or delay < -3600 or delay > 3600 then return end
    -- A slow settings/download response must not undo an edit made in MPV.
    if not edited then mp.set_property_number('sub-delay', delay) end
    if not visibility_ready then
        preferred_track = saved_track or ''
        apply_visibility(preferred_track ~= 'off')
        visibility_ready = true
    end
    if path == '' then return end
    local force = force_enable == 'true'
    if key and key ~= '' then track_keys[path] = key end
    for _, track in ipairs(mp.get_property_native('track-list', {})) do
        if track.type == 'sub' and track['external-filename'] == path then
            if force then
                subtitles_disabled = false
                mp.set_property_native('sub-visibility', true)
                mp.set_property_native('sid', track.id)
                remember_visibility(true)
            end
            if not force then restore_track(); snapshot_selection() end
            publish_downloaded_subtitle()
            show_status('existing')
            return
        end
    end
    if key and key:match('^download:') then title = '已下载字幕'
    elseif key and key:match('^local:') then title = '同目录字幕' end
    local select = force or (not subtitles_disabled and not has_subtitle())
    if not mp.commandv('sub-add', path, select and 'select' or 'auto', title, language) then
        show_status('error')
        return
    end
    if force then
        subtitles_disabled = false
        mp.set_property_native('sub-visibility', true)
    end
    if subtitles_disabled then
        -- Cached/local tracks restore the remembered state; fresh downloads open.
        mp.set_property_native('sid', false)
    end
    if force then remember_visibility(true)
    else restore_track(); snapshot_selection() end
    publish_downloaded_subtitle()
    show_status(status == 'existing' and 'existing' or 'ready')
end)

local function open_offset_settings()
    if input_open then return end
    if not editable() then
        mp.osd_message('请先播放 JavBoss 中的视频', 3)
        return
    end
    if saving then
        mp.osd_message('正在保存字幕偏移…', 3)
        return
    end
    local expected, entry = generation, current_entry()
    input_open = true
    offset_input.get({
        default_text = string.format('%.3f', mp.get_property_number('sub-delay', 0))
            :gsub('0+$', ''):gsub('%.$', ''):gsub('^-0$', '0'),
        closed = function() input_open = false end,
        submit = function(text)
            if not loaded or generation ~= expected or current_entry() ~= entry then
                close_input()
                return
            end
            local seconds = tonumber(text)
            if not seconds or seconds ~= seconds or seconds < -3600 or seconds > 3600 then
                offset_input.log_error('请输入 -3600 到 3600 之间的秒数，可带小数。')
                return
            end
            local ms = math.floor(seconds * 1000 + 0.5)
            edited, saving = true, true
            revision = revision + 1
            mp.set_property_number('sub-delay', ms / 1000)
            close_input()
            mp.osd_message(string.format('字幕偏移已设为 %+.3f 秒，正在保存…', ms / 1000), 3)
            mp.commandv('script-message', 'javboss-subtitle-save', tostring(expected), tostring(entry), tostring(revision), tostring(ms))
        end,
    })
end

mp.register_script_message('javboss-subtitle-offset', open_offset_settings)

local function subtitle_action(choice)
    if choice == 'off' then
        apply_visibility(false)
        visibility_ready = true
        mp.set_property_native('secondary-sid', false)
        remember_visibility(false)
        mp.osd_message('字幕已关闭', 3)
    elseif choice == 'offset' then
        open_offset_settings()
    elseif choice == 'find' then
        if mp.get_property_native('user-data/javboss/subtitle-status') == 'searching' then return end
        mp.commandv('script-message', 'javboss-subtitle-load', tostring(generation), tostring(current_entry()))
    else
        for _, track in ipairs(mp.get_property_native('track-list', {})) do
            if track.type == 'sub' and track_key(track) == choice then
                subtitles_disabled, visibility_ready = false, true
                mp.set_property_native('sub-visibility', true)
                mp.set_property_native('sid', track.id)
                remember_visibility(true)
                break
            end
        end
    end
    publish_menu()
end

mp.register_script_message('javboss-subtitle-action', function(choice, expected, entry)
    if loaded and generation == tonumber(expected) and current_entry() == tonumber(entry) then
        subtitle_action(choice)
    end
end)

mp.register_script_message('javboss-subtitle-selection-error', function(expected, entry)
    if loaded and generation == tonumber(expected) and current_entry() == tonumber(entry) then
        mp.osd_message('字幕选择已生效，但保存失败。', 5)
    end
end)

mp.register_script_message('javboss-subtitle-menu', function()
    if not loaded or input_open then return end
    local expected, entry = generation, current_entry()
    local action
    input_open = true
    local choices, items = menu_items(), {}
    for _, item in ipairs(choices) do items[#items + 1] = item.label end
    input.select({
        prompt = '字幕',
        items = items,
        default_item = 1,
        keep_open = true,
        closed = function()
            input_open = false
            if not action then return end
            -- MPV 0.41 unregisters the previous input handler after closed returns.
            -- Open the next menu/input only after that cleanup has finished.
            mp.add_timeout(0, function()
                if loaded and generation == expected and current_entry() == entry then action() end
            end)
        end,
        submit = function(choice)
            if not loaded or generation ~= expected or current_entry() ~= entry then
                close_input()
                return
            end
            if choices[choice] then action = function() subtitle_action(choices[choice].action) end end
            close_input()
        end,
    })
end)

mp.register_script_message('javboss-subtitle-saved', function(expected, entry, edit, status)
    if not loaded or generation ~= tonumber(expected) or current_entry() ~= tonumber(entry) or revision ~= tonumber(edit) then return end
    saving = false
    if status == 'saved' then
        mp.osd_message(string.format('字幕偏移已保存：%+.3f 秒', mp.get_property_number('sub-delay', 0)), 3)
    else
        mp.osd_message('字幕偏移已生效，但保存失败。请从字幕菜单重新设置。', 5)
    end
end)
