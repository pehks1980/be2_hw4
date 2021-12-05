math.randomseed(os.time())
request = function()
    local k = math.random(0, 1000) local t
    if k > 950 then
        t = "incorrect_admin_token"
    else
        t = "admin_secret_token"
    end
    local url = "/entity"
    wrk.method = "POST"
    wrk.body   = "id=" ..k.. "&data=" ..t
    wrk.headers["Content-Type"] = "application/x-www-form-urlencoded"
    return wrk.format("POST", url)
end