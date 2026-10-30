-- json: read and write JSON.
--
-- local json = require 'json'
-- json.decode(text)          a Lua value: objects and arrays become tables,
--                            marked as json.object and json.array mark
--                            them, and null becomes json.null
-- json.encode(value[, indent])  JSON text; a table marked by json.object or
--                            json.array is that, and otherwise a table with
--                            only the keys 1 to n is an array and any other
--                            an object, with its keys in order; an empty
--                            table unmarked is an array. indent gives
--                            pretty output
-- json.object([t])           marks t, or a new table, as an object and
--                            returns it, so that it is {} when empty
-- json.array([t])            marks t, or a new table, as an array
--
-- Part of Dinah's /lib. A release replaces it only when you have not changed it.

local json = {}

json.null = setmetatable({}, { __tostring = function() return 'null' end })

-- The marks are metatables of their own, so a table that already has a
-- metatable is left as it is and encoded by its keys.
local object_mark, array_mark = { __jsontype = 'object' }, { __jsontype = 'array' }
function json.object(t)
  t = t or {}
  if getmetatable(t) == nil then setmetatable(t, object_mark) end
  return t
end
function json.array(t)
  t = t or {}
  if getmetatable(t) == nil then setmetatable(t, array_mark) end
  return t
end

local escapes = { ['"'] = '\\"', ['\\'] = '\\\\', ['\b'] = '\\b', ['\f'] = '\\f', ['\n'] = '\\n', ['\r'] = '\\r', ['\t'] = '\\t' }

local function encode_string(s)
  return '"' .. s:gsub('[%c"\\]', function(c)
    return escapes[c] or string.format('\\u%04x', c:byte())
  end) .. '"'
end

local function is_array(t)
  local n = 0
  for _ in pairs(t) do n = n + 1 end
  for i = 1, n do
    if t[i] == nil then return false end
  end
  return true, n
end

local function encode(value, indent, depth, seen)
  local kind = type(value)
  if value == nil or value == json.null then return 'null' end
  if kind == 'boolean' then return tostring(value) end
  if kind == 'number' then
    if value ~= value or value == math.huge or value == -math.huge then error('json: cannot encode ' .. tostring(value)) end
    if math.type(value) == 'integer' then return tostring(value) end
    return string.format('%.14g', value)
  end
  if kind == 'string' then return encode_string(value) end
  if kind ~= 'table' then error('json: cannot encode a ' .. kind) end
  if seen[value] then error('json: a table contains itself') end
  seen[value] = true
  local pad, inner, sep, colon = '', '', ',', ':'
  if indent then
    pad = '\n' .. string.rep(indent, depth)
    inner = '\n' .. string.rep(indent, depth + 1)
    colon = ': '
  end
  local parts = {}
  local array, n = is_array(value)
  local mark = getmetatable(value)
  if mark == object_mark then array = false elseif mark == array_mark then array, n = true, #value end
  local out
  if array then
    if n == 0 then out = '[]' else
      for i = 1, n do parts[i] = inner .. encode(value[i], indent, depth + 1, seen) end
      out = '[' .. table.concat(parts, sep) .. pad .. ']'
    end
  else
    local keys = {}
    for k in pairs(value) do
      if type(k) ~= 'string' and type(k) ~= 'number' then error('json: an object key must be a string') end
      keys[#keys + 1] = tostring(k)
    end
    table.sort(keys)
    for i, k in ipairs(keys) do
      local v = value[k]
      if v == nil then v = value[tonumber(k)] end
      parts[i] = inner .. encode_string(k) .. colon .. encode(v, indent, depth + 1, seen)
    end
    out = #parts == 0 and '{}' or '{' .. table.concat(parts, sep) .. pad .. '}'
  end
  seen[value] = nil
  return out
end

function json.encode(value, indent)
  if indent == true then indent = '  ' elseif type(indent) == 'number' then indent = string.rep(' ', indent) end
  return encode(value, indent, 0, {})
end

local function decode_error(text, at, what)
  local line = select(2, text:sub(1, at):gsub('\n', '')) + 1
  error(string.format('json: %s at line %d', what, line), 0)
end

local function skip(text, at)
  return text:find('[^ \t\r\n]', at) or #text + 1
end

local decode_value

local function decode_string(text, at)
  local parts, i = {}, at + 1
  while true do
    local c = text:sub(i, i)
    if c == '' then decode_error(text, i, 'a string is not closed') end
    if c == '"' then return table.concat(parts), i + 1 end
    if c == '\\' then
      local e = text:sub(i + 1, i + 1)
      local simple = { b = '\b', f = '\f', n = '\n', r = '\r', t = '\t', ['"'] = '"', ['\\'] = '\\', ['/'] = '/' }
      if simple[e] then parts[#parts + 1] = simple[e]; i = i + 2
      elseif e == 'u' then
        local hex = text:sub(i + 2, i + 5)
        if not hex:match('^%x%x%x%x$') then decode_error(text, i, 'a \\u escape is not four hex digits') end
        local code = tonumber(hex, 16)
        i = i + 6
        if code >= 0xD800 and code <= 0xDBFF and text:sub(i, i + 1) == '\\u' then
          local low = tonumber(text:sub(i + 2, i + 5), 16)
          if low and low >= 0xDC00 and low <= 0xDFFF then
            code = 0x10000 + (code - 0xD800) * 0x400 + (low - 0xDC00)
            i = i + 6
          end
        end
        parts[#parts + 1] = utf8.char(code)
      else decode_error(text, i, 'an unknown escape') end
    else
      local j = text:find('["\\]', i) or #text + 1
      parts[#parts + 1] = text:sub(i, j - 1)
      i = j
    end
  end
end

function decode_value(text, at)
  at = skip(text, at)
  local c = text:sub(at, at)
  if c == '{' then
    local obj = setmetatable({}, object_mark)
    at = skip(text, at + 1)
    if text:sub(at, at) == '}' then return obj, at + 1 end
    while true do
      if text:sub(at, at) ~= '"' then decode_error(text, at, 'an object key must be a string') end
      local key
      key, at = decode_string(text, at)
      at = skip(text, at)
      if text:sub(at, at) ~= ':' then decode_error(text, at, 'a colon is missing') end
      obj[key], at = decode_value(text, at + 1)
      at = skip(text, at)
      local d = text:sub(at, at)
      if d == '}' then return obj, at + 1 end
      if d ~= ',' then decode_error(text, at, 'a comma or } is missing') end
      at = skip(text, at + 1)
    end
  elseif c == '[' then
    local arr, n = setmetatable({}, array_mark), 0
    at = skip(text, at + 1)
    if text:sub(at, at) == ']' then return arr, at + 1 end
    while true do
      n = n + 1
      arr[n], at = decode_value(text, at)
      at = skip(text, at)
      local d = text:sub(at, at)
      if d == ']' then return arr, at + 1 end
      if d ~= ',' then decode_error(text, at, 'a comma or ] is missing') end
      at = at + 1
    end
  elseif c == '"' then
    return decode_string(text, at)
  elseif text:sub(at, at + 3) == 'true' then return true, at + 4
  elseif text:sub(at, at + 4) == 'false' then return false, at + 5
  elseif text:sub(at, at + 3) == 'null' then return json.null, at + 4
  else
    local num = text:match('^-?%d+%.?%d*[eE]?[-+]?%d*', at)
    if not num or num == '' or num == '-' then decode_error(text, at, 'a value is expected') end
    return math.tointeger(tonumber(num)) or tonumber(num), at + #num
  end
end

function json.decode(text)
  if type(text) ~= 'string' then error('json.decode needs text', 2) end
  local value, at = decode_value(text, 1)
  at = skip(text, at)
  if at <= #text then decode_error(text, at, 'there is text after the value') end
  return value
end

return json
