package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Payload struct {
	SecretKey       string    `json:"secret_key"`
	OrderID         int       `json:"order_id"`
	ItemID          int       `json:"item_id,omitempty"`
	ItemIDs         []int     `json:"item_ids,omitempty"`
	MassDrone       float64   `json:"mass_drone"`
	MassPayload     float64   `json:"mass_payload"`
	BatteryEnergyWh float64   `json:"battery_energy_Wh"`
	Multiplier      float64   `json:"multiplier"`
	WindCoeff       float64   `json:"wind_coeff"`
	RainCoeff       float64   `json:"rain_coeff"`
}

type ResultPayload struct {
	SecretKey string  `json:"secret_key"`
	OrderID   int     `json:"order_id"`
	ItemID    int     `json:"item_id"`
	Runtime   float64 `json:"runtime"`
	Status    string  `json:"status"`
}

func main() {
	rand.Seed(time.Now().UnixNano())

	callback := os.Getenv("DJANGO_CALLBACK_URL")
	if callback == "" {
		callback = "http://127.0.0.1:8000/api/drone_items/async/update_results/"
	}
	secret := os.Getenv("ASYNC_SECRET_KEY")
	if secret == "" {
		secret = "ABC123XYZ"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/api/drone_process", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var p Payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		itemList := p.ItemIDs
		if len(itemList) == 0 && p.ItemID != 0 {
			itemList = []int{p.ItemID}
		}

		if len(itemList) == 0 {
			http.Error(w, "no item_ids provided", http.StatusBadRequest)
			return
		}

		for _, itemID := range itemList {
			go handleSingleItem(p, itemID, callback, secret)
		}

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"queued"}`))
	})

	log.Printf("async service listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleSingleItem(p Payload, itemID int, callback, secret string) {
	delay := 5 + rand.Intn(6)
	time.Sleep(time.Duration(delay) * time.Second)

	totalMass := p.MassDrone + p.MassPayload
	if totalMass <= 0 {
		totalMass = 1.0
	}

	power := p.Multiplier * math.Pow(totalMass, 1.5) * p.WindCoeff * p.RainCoeff

	var runtime float64
	var status string
	if power <= 0 {
		runtime = 0
		status = "zero_power"
	} else {
		runtime = (p.BatteryEnergyWh / power) * 60.0
		status = "success"
		//noise := 0.9 + rand.Float64()*0.2
		//runtime = runtime * noise
	}

	result := ResultPayload{
		SecretKey: secret,
		OrderID:   p.OrderID,
		ItemID:    itemID,
		Runtime:   runtime,
		Status:    status,
	}

	body, _ := json.Marshal(result)
	req, err := http.NewRequest("POST", callback, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[callback err] item %d: %v\n", itemID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[callback do err] item %d: %v\n", itemID, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Callback for order %d item %d returned %s\n", p.OrderID, itemID, strconv.Itoa(resp.StatusCode))
}
