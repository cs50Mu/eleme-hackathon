local food_id = ARGV[1]
local food_cnt = ARGV[2]
local cart_id = ARGV[3]
local user_id = ARGV[4]

if redis.call("sismember", "foods", food_id) ~= 1 then
    return -1
end

if redis.call("sismember", "carts", cart_id) ~= 1 then
    return -2
end

local user_cart = user_id .. "." .. cart_id

if redis.call("exists", user_cart) ~= 1 then
    return -3
end

local food_total = redis.call("hget", user_cart, "total")

if food_total + food_cnt > 3 then
    return -4
end

local foodid_key = "id:" .. food_id
redis.call("hincrby", user_cart, foodid_key, food_cnt)
redis.call("hincrby", user_cart, "total", food_cnt)
return 0
