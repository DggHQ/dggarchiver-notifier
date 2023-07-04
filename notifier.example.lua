local http = require("http")
local inspect = require("inspect")

ReceiveResponse = {}

local client = http.client()

local pushoverToken = "xxx"
local pushoverUser = "xxx"

local char_to_hex = function(c)
	return string.format("%%%02X", string.byte(c))
end

local function urlencode(url)
	if url == nil then
		return
	end
	url = url:gsub("\n", "\r\n")
	url = url:gsub("([^%w ])", char_to_hex)
	url = url:gsub(" ", "+")
	return url
end

function OnReceive(vod)
	local urlParams = string.format("?token=%s&user=%s&message=%s&title=%s&priority=-2", pushoverToken, pushoverUser, vod,
		urlencode("Found a currently running stream..."))
	local request = http.request("POST", "https://api.pushover.net/1/messages.json" .. urlParams)
	local result, err = client:do_request(request)
	if err then
		error(err)
		ReceiveResponse.filled = true
		ReceiveResponse.error = true
		ReceiveResponse.message = err
		return
	end
	if not (result.code == 200) then
		ReceiveResponse.filled = true
		ReceiveResponse.error = true
		ReceiveResponse.message = tostring(inspect(result))
		return
	end
	ReceiveResponse.filled = true
	ReceiveResponse.error = false
	ReceiveResponse.message = "Sent a notification to Pushover successfully"
end