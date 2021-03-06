package service

import (
	"time"

	"github.com/dgrijalva/jwt-go"
)

// 生成token时需要的密钥，这个是jwt中最重要的东西千万不能泄露!!!
var secretKey = []byte("abcd1234!@#$")

// ArithmeticCustomClaims 自定义声明
type ArithmeticCustomClaims struct {
	UserId string `json:"userId"`
	Name   string `json:"name"`

	jwt.StandardClaims
}

// jwtKeyFunc 返回密钥
func JwtKeyFunc(token *jwt.Token) (interface{}, error) {
	return secretKey, nil
}

// Sign 生成token
func Sign(name, uid string) (string, error) {

	//为了演示方便，设置两分钟后过期
	expAt := time.Now().Add(time.Duration(2) * time.Minute).Unix()

	// 创建声明
	claims := ArithmeticCustomClaims{
		UserId: uid,
		Name:   name,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expAt,
			Issuer:    "system",
		},
	}

	//创建token，指定加密算法为HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	//生成token
	return token.SignedString(secretKey)
}
