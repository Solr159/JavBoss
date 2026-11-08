-- MPV 0.41's text console is anchored at the bottom left. This small numeric
-- dialog supplies a centered input while retaining the subtitle save callbacks.
local assdraw = require 'mp.assdraw'
local dialog, overlay
local bindings = {}
local api = {}
local render

function api.terminate()
    if not dialog then return end
    local closed = dialog.closed
    dialog = nil
    for _, name in ipairs(bindings) do mp.remove_key_binding(name) end
    bindings = {}
    overlay:remove()
    mp.set_property_native('user-data/javboss/subtitle-offset-open', false)
    if closed then closed() end
end

local function replace_selection(text)
    if not dialog then return end
    if not text:match('^[%d%+%-%.]*$') then return end
    local before = dialog.selected and '' or dialog.text:sub(1, dialog.cursor)
    local after = dialog.selected and '' or dialog.text:sub(dialog.cursor + 1)
    if #before + #text + #after > 24 then return end
    dialog.text, dialog.cursor = before .. text .. after, #before + #text
    dialog.selected, dialog.error = false, nil
    render()
end

local function submit()
    if dialog then dialog.submit(dialog.text) end
end

render = function()
    if not dialog then return end
    local w, h = mp.get_osd_size()
    if w <= 0 or h <= 0 then w, h = 960, 540 end
    -- Match the native selector's font size, padding, translucent background
    -- and window scaling, keeping only the prompt and editable number.
    local scale = mp.get_property_native('osd-scale-by-window', true) and h / 720
        or mp.get_property_number('display-hidpi-scale', 1)
    scale = math.min(scale, w / 460, h / 166)
    local width, height = 440 * scale, (dialog.error and 142 or 108) * scale
    local x, y = (w - width) / 2, (h - height) / 2
    dialog.bounds = {x, y, x + width, y + height}
    local ass = assdraw.ass_new()
    ass:new_event()
    ass:pos(0, 0)
    ass:an(7)
    ass:append('{\\bord0\\shad0\\blur0\\1c&H000000&\\1a&H50&}')
    ass:draw_start()
    ass:round_rect_cw(x, y, x + width, y + height, 8 * scale)
    ass:draw_stop()
    local function text(top, size, value, color)
        ass:new_event()
        ass:pos(x + 10 * scale, y + top * scale)
        ass:an(7)
        ass:append(string.format('{\\bord0\\shad0\\blur0\\fs%f\\1a&H00&\\1c&H%s&}', size * scale, color or 'FFFFFF'))
        ass:append(value)
    end
    text(10, 24, '字幕偏移（秒）：')
    text(40, 16, '输入完成后按 Enter 保存', 'CCCCCC')
    local escaped = function(value) return mp.command_native({'escape-ass', value}) end
    local value = dialog.selected and '{\\u1}' .. escaped(dialog.text) .. '{\\u0}│'
        or escaped(dialog.text:sub(1, dialog.cursor)) .. '│' .. escaped(dialog.text:sub(dialog.cursor + 1))
    text(70, 24, value)
    if dialog.error then text(108, 16, escaped(dialog.error), '8888FF') end
    overlay.res_x, overlay.res_y, overlay.z, overlay.data = w, h, 2000, ass.text
    overlay:update()
end

function api.get(options)
    api.terminate()
    overlay = overlay or mp.create_osd_overlay('ass-events')
    dialog = {text = options.default_text or '', selected = true, submit = options.submit, closed = options.closed}
    dialog.cursor = #dialog.text
    local function bind(key, handler, opts)
        local name = 'javboss-offset-' .. (#bindings + 1)
        bindings[#bindings + 1] = name
        mp.add_forced_key_binding(key, name, handler, opts)
    end
    bind('any_unicode', function(event)
        if event.key_text and (event.event == 'press' or event.event == 'down' or event.event == 'repeat') then
            replace_selection(event.key_text)
        end
    end, {complex = true, repeatable = true})
    for i = 0, 9 do
        bind('KP' .. i, function() replace_selection(tostring(i)) end, {repeatable = true})
    end
    for key, value in pairs({KP_DEC = '.', KP_ADD = '+', KP_SUBTRACT = '-'}) do
        bind(key, function() replace_selection(value) end, {repeatable = true})
    end
    bind('ESC', api.terminate)
    bind('ENTER', submit)
    bind('KP_ENTER', submit)
    bind('Ctrl+a', function() dialog.selected = true; render() end)
    bind('Ctrl+u', function() dialog.selected = true; replace_selection('') end)
    bind('Ctrl+v', function() replace_selection(mp.get_property('clipboard/text', '')) end)
    for _, key in ipairs({'LEFT', 'RIGHT', 'HOME', 'END'}) do
        bind(key, function()
            if key == 'HOME' then dialog.cursor = 0
            elseif key == 'END' then dialog.cursor = #dialog.text
            elseif key == 'LEFT' then dialog.cursor = dialog.selected and 0 or math.max(0, dialog.cursor - 1)
            else dialog.cursor = dialog.selected and #dialog.text or math.min(#dialog.text, dialog.cursor + 1) end
            dialog.selected = false
            render()
        end, {repeatable = true})
    end
    for _, key in ipairs({'BS', 'DEL'}) do
        bind(key, function()
            if dialog.selected then replace_selection(''); return end
            local at = key == 'BS' and dialog.cursor or dialog.cursor + 1
            if at > 0 then
                dialog.text = dialog.text:sub(1, at - 1) .. dialog.text:sub(at + 1)
                if key == 'BS' then dialog.cursor = math.max(0, dialog.cursor - 1) end
            end
            dialog.error = nil
            render()
        end, {repeatable = true})
    end
    -- Close on release so the entire outside click is consumed by the dialog.
    bind('MBTN_LEFT', function(event)
        if not dialog or (event.event ~= 'up' and event.event ~= 'press') then return end
        local x, y = mp.get_mouse_pos()
        local bounds = dialog.bounds
        if bounds and (x < bounds[1] or y < bounds[2] or x > bounds[3] or y > bounds[4]) then
            api.terminate()
        end
    end, {complex = true})
    bind('MBTN_LEFT_DBL', function() end)
    mp.set_property_native('user-data/javboss/subtitle-offset-open', true)
    render()
end

function api.log_error(message)
    if dialog then dialog.error = message; render() end
end

mp.observe_property('osd-dimensions', 'native', render)
return api
