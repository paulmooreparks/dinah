-- cli: what the standard scripts in /bin share.
--
-- local cli = require 'cli'
-- cli.input(files)      the text of the files, one after another, or what was
--                       piped in when there are none; stops the script with a
--                       message when a file cannot be read or there is nothing
-- cli.lines(text)       the lines of a text, without their line ends
-- cli.options(args, spec)  splits options from operands: spec maps each
--                       option letter to true when it takes a value; returns
--                       the options, by letter, and the operands. As POSIX
--                       utilities do, options come first: the first operand,
--                       or --, ends them
-- cli.fail(name, message[, status])  writes "name: message" and stops the
--                       script with the status (1 unless given)
--
-- Part of Dinah's /lib. A release replaces it only when you have not changed it.

local cli = {}

function cli.fail(name, message, status)
  io.stderr:write(name .. ': ' .. message .. '\n')
  os.exit(status or 1)
end

function cli.input(files, name)
  name = name or arg[0]
  if not files or #files == 0 then
    local text = io.read('a')
    if text == nil or text == '' then
      if text == '' then return '' end
      cli.fail(name, 'which file? (or pipe text into it)')
    end
    return text
  end
  local parts = {}
  for i, path in ipairs(files) do
    local f, err = io.open(path, 'r')
    if not f then cli.fail(name, err) end
    parts[i] = f:read('a') or ''
    f:close()
  end
  return table.concat(parts)
end

function cli.lines(text)
  local out = {}
  if text == '' then return out end
  for line in (text:sub(-1) == '\n' and text or text .. '\n'):gmatch('(.-)\r?\n') do out[#out + 1] = line end
  return out
end

function cli.options(args, spec, name)
  name = name or arg[0]
  local opts, rest, i = {}, {}, 1
  while i <= #args do
    local a = args[i]
    if a == '--' then
      for j = i + 1, #args do rest[#rest + 1] = args[j] end
      break
    elseif a:match('^%-%a') and not tonumber(a) then
      local k = 2
      while k <= #a do
        local letter = a:sub(k, k)
        if spec[letter] == nil then cli.fail(name, 'unknown option -' .. letter .. '; "man ' .. name .. '" lists them', 2) end
        if spec[letter] then
          local value = a:sub(k + 1)
          if value == '' then i = i + 1 value = args[i] end
          if value == nil then cli.fail(name, '-' .. letter .. ' needs a value', 2) end
          opts[letter] = value
          break
        end
        opts[letter] = true
        k = k + 1
      end
    else
      for j = i, #args do rest[#rest + 1] = args[j] end
      break
    end
    i = i + 1
  end
  return opts, rest
end

return cli
