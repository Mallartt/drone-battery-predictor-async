package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	djangoURL = "http://localhost:8000/api/drone_orders/%d/calculated_result/"
	secretKey = "ABC123XYZ"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	r := gin.Default()

	r.POST("/api/drone_async", func(c *gin.Context) {
		var payload struct {
			OrderID int `json:"order_id"`
		}
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		go handleAsync(payload.OrderID)

		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"order_id": payload.OrderID,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Async service started on port %s", port)
	r.Run(":" + port)
}

func handleAsync(orderID int) {
	delay := time.Duration(5+rand.Intn(6)) * time.Second
	log.Printf("Processing order %d for %v...", orderID, delay)
	time.Sleep(delay)

	result := "FAIL"
	if rand.Float64() > 0.5 {
		result = "SUCCESS"
	}

	body := map[string]string{
		"key":    secretKey,
		"result": result,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return
	}

	url := fmt.Sprintf(djangoURL, orderID)

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending result to Django: %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	log.Printf("Sent result '%s' for order %d, response code: %d, body: %s", result, orderID, resp.StatusCode, string(respBody))
}
