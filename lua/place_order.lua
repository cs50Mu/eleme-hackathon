local user_id = ARGV[1]
local cart_id = ARGV[2]
local order_id = ARGV[3]


if redis.call("sismember", "carts", cart_id) ~= 1 then
    -- 篮子不存在
    return -1
end

local user_cart = user_id .. "." .. cart_id

if redis.call("exists", user_cart) ~= 1 then
    -- 无权限访问指定的篮子
    return -2
end

if redis.call("hexists", "orders", user_id) == 1 then
    -- 每个用户只能下一单
    return -3
end

local function split(s, delimiter)
    local result = {};
    for match in (s..delimiter):gmatch("(.-)"..delimiter) do
        table.insert(result, match);
    end
    return result;
end

local cart_foods = redis.call("hgetall", user_cart)
-- #是用于获取数组的长度
for id = 1, #cart_foods, 2 do
    if cart_foods[id] ~= "total" then
        local splitted = split(cart_foods[id], ":")
        local food_id = splitted[2]
        local food_key = "food:" .. food_id
        local food_stock = redis.call("hget", food_key, "stock")
        local food_cnt = cart_foods[id+1]
        if food_cnt > food_stock then
            -- 库存不足
            return -4
        end
        redis.call("hincrby", food_key, "stock", -food_cnt)
        local order_key = "order:" .. order_id
        redis.call("hset", order_key, food_id, food_cnt)
    end
end
return 0
