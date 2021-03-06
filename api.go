package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	logging "github.com/op/go-logging"
	uuid "github.com/satori/go.uuid"
	//"github.com/op/go-logging"
	//"github.com/pkg/errors"
	//"gopkg.in/go-playground/validator.v8"
)

var logger = logging.MustGetLogger("api")

var mu sync.Mutex

const maxFoodsInCart = 3

var scriptSha string

// health check
func checkHealth(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}

type loginForm struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

var redisClient *redis.Client
var scriptMapping map[string]string

func init() {
	redisClient = redis.NewClient(&redis.Options{
		//Addr: "127.0.0.1:6379",
		Addr:     "10.0.2.2:6379",
		Password: "",
		DB:       1,
	})

	// load lua scripts
	scriptMapping = make(map[string]string)
	loadLuaScript("lua/add_food.lua", "add_food")
	loadLuaScript("lua/place_order.lua", "place_order")
}

func loadLuaScript(scriptPath, scriptName string) {
	script := readFile(scriptPath)
	scriptSha, _ = redisClient.ScriptLoad(script).Result()
	scriptMapping[scriptName] = scriptSha
}

func readFile(filename string) string {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return string(content)
}

func validateBody(data []byte, v interface{}) (bool, map[string]string) {
	if string(data) == "" {
		return false, map[string]string{"code": "EMPTY_REQUEST",
			"message": "请求体为空"}
	}
	err := json.Unmarshal(data, v)
	if err != nil {
		return false, map[string]string{"code": "MALFORMED_JSON",
			"message": "格式错误"}
	}
	return true, nil
}

// middleware, validate access_token
func authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		accessToken := c.DefaultQuery("access_token", "")
		if accessToken == "" {
			accessToken = c.GetHeader("Access-Token")
		}
		if accessToken != "" {
			decoded, _ := base64.URLEncoding.DecodeString(accessToken)
			splitted := strings.Split(string(decoded), ".")
			userID, sign := splitted[0], splitted[1]
			h := md5.New()
			io.WriteString(h, userID)
			calcedSign := hex.EncodeToString(h.Sum(nil))
			if calcedSign != sign {
				c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_ACCESS_TOKEN",
					"message": "无效的令牌"})
				c.Abort()
			}
			c.Set("userID", userID)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_ACCESS_TOKEN",
				"message": "无效的令牌"})
			//return
			c.Abort()
		}
	}
}

// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func randStr(n int) string {
	return uuid.Must(uuid.NewV1()).String()
}

// login
func login(c *gin.Context) {
	var loginData loginForm
	body, _ := c.GetRawData()
	isValid, respMessage := validateBody(body, &loginData)
	if !isValid {
		c.JSON(http.StatusBadRequest, respMessage)
		return
	}
	idPass, err := redisClient.HGet("user:pass", loginData.UserName).Result()
	if err != nil {
		if err == redis.Nil {
			c.JSON(http.StatusForbidden, gin.H{"code": "USER_AUTH_FAIL",
				"message": "用户名或密码错误"})
			return
		}
		panic(err)
	}
	splitted := strings.Split(idPass, ":")
	userIDStr, pass := splitted[0], splitted[1]
	if pass != loginData.Password {
		c.JSON(http.StatusForbidden, gin.H{"code": "USER_AUTH_FAIL",
			"message": "用户名或密码错误"})
		return
	}
	h := md5.New()
	io.WriteString(h, userIDStr)
	composit := userIDStr + "." + hex.EncodeToString(h.Sum(nil))
	accessToken := base64.URLEncoding.EncodeToString([]byte(composit))
	userID, _ := strconv.Atoi(userIDStr)
	c.JSON(http.StatusOK, gin.H{"user_id": userID,
		"username":     loginData.UserName,
		"access_token": accessToken})
}

// get foods
func getFoods(c *gin.Context) {
	//userName := c.MustGet("userName").(string)
	// return foods
	//	foodIDs, err := redisClient.SMembers("foods").Result()
	//	if err != nil {
	//		panic(err)
	//	}
	//	var res []map[string]int
	//	for _, foodID := range foodIDs {
	//		foodKey := fmt.Sprintf("food:%s", foodID)
	//		foodMap, err := redisClient.HGetAll(foodKey).Result()
	//		if err != nil {
	//			panic(err)
	//		}
	//		priceStr, stockStr := foodMap["price"], foodMap["stock"]
	//		id, _ := strconv.Atoi(foodID)
	//		price, _ := strconv.Atoi(priceStr)
	//		stock, _ := strconv.Atoi(stockStr)
	//		foodObj := map[string]int{
	//			"id":    id,
	//			"price": price,
	//			"stock": stock,
	//		}
	//		res = append(res, foodObj)
	//	}
	c.JSON(http.StatusOK, "")
}

func generateCartID() string {
	return randStr(10)
}

// 	create cart
func createCart(c *gin.Context) {
	userID := c.MustGet("userID").(string)
	cartID := generateCartID()
	go func() {
		userCartKey := fmt.Sprintf("%s.%s", userID, cartID)
		// user.cartID
		err := redisClient.HSet(userCartKey, "total", 0).Err()
		if err != nil {
			panic(err)
		}
		// add cartID to set
		err = redisClient.SAdd("carts", cartID).Err()
		if err != nil {
			panic(err)
		}
	}()
	c.JSON(http.StatusOK, gin.H{"cart_id": cartID})
}

type addFoodForm struct {
	FoodID int `json:"food_id"`
	Count  int `json:"count"`
}

// add food
func addFood(c *gin.Context) {
	userID := c.MustGet("userID").(string)
	var addFoodData addFoodForm
	body, _ := c.GetRawData()
	isValid, respMessage := validateBody(body, &addFoodData)
	if !isValid {
		c.JSON(http.StatusBadRequest, respMessage)
		return
	}
	foodID := addFoodData.FoodID
	foodCount := addFoodData.Count
	cartID := c.Param("cart_id")
	keys := make([]string, 0)
	retCode, err := redisClient.EvalSha(scriptMapping["add_food"], keys, foodID, foodCount, cartID, userID).Int64()
	if err != nil {
		panic(err)
	}
	switch retCode {
	case -1:
		c.JSON(http.StatusNotFound, gin.H{"code": "FOOD_NOT_FOUND",
			"message": "食物不存在"})
	case -2:
		c.JSON(http.StatusNotFound, gin.H{"code": "CART_NOT_FOUND",
			"message": "篮子不存在"})
	case -3:
		c.JSON(http.StatusUnauthorized, gin.H{"code": "NOT_AUTHORIZED_TO_ACCESS_CART",
			"message": "无权限访问指定的篮子"})
	case -4:
		c.JSON(http.StatusForbidden, gin.H{"code": "FOOD_OUT_OF_LIMIT",
			"message": "篮子中食物数量超过了三个"})
	default:
		c.JSON(http.StatusNoContent, "")
	}
}

type orderForm struct {
	CartID string `json:"cart_id"`
}

func generateOrderID() string {
	return randStr(10)
}

// place order
func placeOrder(c *gin.Context) {
	userID := c.MustGet("userID").(string)
	var orderData orderForm
	body, _ := c.GetRawData()
	isValid, respMessage := validateBody(body, &orderData)
	if !isValid {
		c.JSON(http.StatusBadRequest, respMessage)
		return
	}
	cartID := orderData.CartID
	orderID := generateOrderID()
	keys := make([]string, 0)
	retCode, err := redisClient.EvalSha(scriptMapping["place_order"], keys, userID, cartID, orderID).Int64()
	if err != nil {
		panic(err)
	}
	switch retCode {
	case -1:
		c.JSON(http.StatusNotFound, gin.H{"code": "CART_NOT_FOUND",
			"message": "篮子不存在"})
	case -2:
		c.JSON(http.StatusUnauthorized, gin.H{"code": "NOT_AUTHORIZED_TO_ACCESS_CART",
			"message": "无权限访问指定的篮子"})
	case -3:
		c.JSON(http.StatusForbidden, gin.H{"code": "ORDER_OUT_OF_LIMIT",
			"message": "每个用户只能下一单"})
	case -4:
		c.JSON(http.StatusForbidden, gin.H{"code": "FOOD_OUT_OF_STOCK",
			"message": "食物库存不足"})
	default:
		// 成功扣减后再记录订单
		go func() {
			err = redisClient.HSet("orders", userID, orderID).Err()
			if err != nil {
				panic(err)
			}
		}()
		c.JSON(http.StatusOK, gin.H{"id": orderID})
	}
}

// get order
func getOrders(c *gin.Context) {
	type order struct {
		ID         string           `json:"id"`
		Items      []map[string]int `json:"items"`
		PriceTotal int64            `json:"total"`
	}
	resp := make([]order, 0)
	userID := c.MustGet("userID").(string)
	orderID, err := redisClient.HGet("orders", userID).Result()
	if err != nil {
		if err == redis.Nil {
			c.JSON(http.StatusOK, resp)
			return
		}
		panic(err)
	}
	orderKey := fmt.Sprintf("order:%s", orderID)
	kvs, err := redisClient.HGetAll(orderKey).Result()
	if err != nil {
		panic(err)
	}
	var items []map[string]int
	var priceTotal int64
	for k, v := range kvs {
		if k != "total" {
			foodIDStr := k
			foodKey := fmt.Sprintf("food:%s", foodIDStr)
			foodPrice, _ := redisClient.HGet(foodKey, "price").Int64()
			foodID, _ := strconv.Atoi(foodIDStr)
			foodCount, _ := strconv.Atoi(v)
			orderItem := map[string]int{
				"food_id": foodID,
				"count":   foodCount,
			}
			priceTotal += foodPrice * int64(foodCount)
			items = append(items, orderItem)
		}
	}
	foodOrder := order{
		ID:         orderID,
		Items:      items,
		PriceTotal: priceTotal,
	}
	resp = append(resp, foodOrder)
	c.JSON(http.StatusOK, resp)
}

// get all orders
func getAllOrders(c *gin.Context) {
	userID := c.MustGet("userID").(string)
	// root uid
	if userID != "1" {
		c.JSON(http.StatusForbidden, gin.H{"code": "MUST_BE_ROOT",
			"message": "必须为管理员用户"})
		return
	}
	type order struct {
		ID         string           `json:"id"`
		UserID     int              `json:"user_id"`
		Items      []map[string]int `json:"items"`
		PriceTotal int64            `json:"total"`
	}
	var resp []order
	orderMap, err := redisClient.HGetAll("orders").Result()
	if err != nil {
		panic(err)
	}
	for k, v := range orderMap {
		userID = k
		orderID := v
		orderKey := fmt.Sprintf("order:%s", orderID)
		kvs, err := redisClient.HGetAll(orderKey).Result()
		if err != nil {
			panic(err)
		}
		var items []map[string]int
		var priceTotal int64
		for k, v := range kvs {
			if k != "total" {
				foodIDStr := k
				foodKey := fmt.Sprintf("food:%s", foodIDStr)
				foodPrice, _ := redisClient.HGet(foodKey, "price").Int64()
				foodID, _ := strconv.Atoi(foodIDStr)
				foodCount, _ := strconv.Atoi(v)
				orderItem := map[string]int{
					"food_id": foodID,
					"count":   foodCount,
				}
				priceTotal += foodPrice * int64(foodCount)
				items = append(items, orderItem)
			}
		}
		userid, _ := strconv.Atoi(userID)
		foodOrder := order{
			ID:         orderID,
			UserID:     userid,
			Items:      items,
			PriceTotal: priceTotal,
		}
		resp = append(resp, foodOrder)
	}
	c.JSON(http.StatusOK, resp)
}
