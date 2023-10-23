// Sample run-helloworld is a minimal Cloud Run service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	sqlcon "github.com/go-sql-driver/mysql"
	gormcon "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Stock struct {
	gorm.Model
	Id         int       `json:"id" gorm:"primaryKey,autoIncrement,not null"`
	Name       string    `json:"name" gorm:"not null"`
	Amount     int       `json:"amount" gorm:"not null"`
	Created_at time.Time `json:"created_at" gorm:"not null"`
	Updated_at time.Time `json:"updated_at" gorm:"not null"`
}

type StocksResponse struct {
	Id         int       `json:"id" gorm:"column:id"`
	Name       string    `json:"name" gorm:"column:name"`
	Amount     int       `json:"amount" gorm:"column:amount"`
	Created_at time.Time `json:"created_at" gorm:"column:created_at"`
	Updated_at time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func main() {
	log.Print("starting server...")
	http.HandleFunc("/hello", helloHandler)
	http.HandleFunc("/receipt", receiptHandler)
	http.HandleFunc("/shipment", shipmentHandler)
	http.HandleFunc("/get", getHandler)

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	name := os.Getenv("NAME")
	if name == "" {
		name = "World"
	}
	fmt.Fprintf(w, "Hello %s!\n", name)
}

func receiptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db, err := connectWithConnector()
	if err != nil {
		log.Println(err)
		http.Error(w, "Fail to connect db", http.StatusInternalServerError)
		return
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	defer tx.Commit()

	// リクエストボディからJSONデータを読み取り
	var request Stock
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		log.Println(err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if request.Amount <= 0 {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	//テーブル存在チェック
	if db.Migrator().HasTable(&Stock{}) == false {
		//テーブル作成クエリを実行
		if err := db.Migrator().CreateTable(&Stock{}).Error; err != nil {
			log.Println(err())
			http.Error(w, "Fail to create table", http.StatusInternalServerError)
			return
		}
	}

	// JSONデータから名前を取得
	name := request.Name
	amount := request.Amount

	oldStock, err := checkItem(db, name)

	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Println("New Item arrival!")
	} else if err != nil {
		log.Println(err)
		http.Error(w, "Fail to check table", http.StatusInternalServerError)
		return
	}

	if err != nil && strings.Contains(err.Error(), "record not found") {
		if err := insertNewItem(db, name, amount); err != nil {
			log.Println(err)
			http.Error(w, "Fail to insert new item", http.StatusInternalServerError)
		}
	} else {
		amount = amount + oldStock.Amount
		if err := updateItem(db, oldStock.Id, amount); err != nil {
			log.Println(err)
			http.Error(w, "Fail to update new item", http.StatusInternalServerError)
		}
	}

	// レスポンスを生成
	var response StocksResponse
	if err := db.Table("stocks").Where("name = ?", name).Scan(&response).Error; err != nil {
		log.Println(err)
	}

	// レスポンスをクライアントに返す
	w.Header().Set("Content-Type", "json")
	w.WriteHeader(http.StatusOK)
	byteResp, _ := json.Marshal(response)
	w.Write(byteResp)
}

func shipmentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db, err := connectWithConnector()
	if err != nil {
		log.Println(err)
		http.Error(w, "Fail to connect db", http.StatusInternalServerError)
		return
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	defer tx.Commit()

	// リクエストボディからJSONデータを読み取り
	var request Stock
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		log.Println(err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if request.Amount <= 0 {
		http.Error(w, "Invalid Amount", http.StatusBadRequest)
		return
	}

	//テーブル存在チェック
	if db.Migrator().HasTable(&Stock{}) == false {
		//テーブル作成クエリを実行
		if err := db.Migrator().CreateTable(&Stock{}).Error; err != nil {
			log.Println(err())
			http.Error(w, "Fail to create table", http.StatusInternalServerError)
			return
		}
	}

	// JSONデータから名前を取得
	name := request.Name
	amount := request.Amount

	oldStock, err := checkItem(db, name)

	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Println(err)
		http.Error(w, "Invalid Item", http.StatusBadRequest)
		return
	} else if err != nil {
		log.Println(err)
		http.Error(w, "Fail to check table", http.StatusInternalServerError)
		return
	}

	amount = oldStock.Amount - amount
	if amount < 0 {
		http.Error(w, "Invalid Amount", http.StatusBadRequest)
		return
	}
	if err := updateItem(db, oldStock.Id, amount); err != nil {
		log.Println(err)
		http.Error(w, "Fail to update new item", http.StatusInternalServerError)
	}

	// レスポンスを生成
	var response StocksResponse
	if err := db.Table("stocks").Where("name = ?", name).Scan(&response).Error; err != nil {
		log.Println(err)
	}

	// レスポンスをクライアントに返す
	w.Header().Set("Content-Type", "json")
	w.WriteHeader(http.StatusOK)
	byteResp, _ := json.Marshal(response)
	w.Write(byteResp)
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	var stocks []StocksResponse
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db, err := connectWithConnector()
	if err != nil {
		log.Println(err)
		http.Error(w, "Fail to connect db", http.StatusInternalServerError)
		return
	}
	result := db.Table("stocks").Find(&stocks)
	if result.Error != nil {
		log.Println(err)
		http.Error(w, "Fail to connect db", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "json")
	w.WriteHeader(http.StatusOK)
	response, _ := json.Marshal(stocks)
	w.Write(response)
}

func connectWithConnector() (*gorm.DB, error) {
	mustGetenv := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			log.Fatalf("Fatal Error in connect_connector.go: %s environment variable not set.", k)
		}
		return v
	}

	dbUser := mustGetenv("DB_USER")                                  // e.g. 'my-db-user'
	dbPwd := mustGetenv("DB_PASS")                                   // e.g. 'my-db-password'
	dbName := mustGetenv("DB_NAME")                                  // e.g. 'my-database'
	instanceConnectionName := mustGetenv("INSTANCE_CONNECTION_NAME") // e.g. 'project:region:instance'

	d, err := cloudsqlconn.NewDialer(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cloudsqlconn.NewDialer: %w", err)
	}
	var opts []cloudsqlconn.DialOption

	sqlcon.RegisterDialContext("cloudsqlconn",
		func(ctx context.Context, addr string) (net.Conn, error) {
			return d.Dial(ctx, instanceConnectionName, opts...)
		})
	dbURI := fmt.Sprintf("%s:%s@cloudsqlconn(localhost:3306)/%s?parseTime=true&loc=Asia%%2FTokyo",
		dbUser, dbPwd, dbName)

	db, err := gorm.Open(gormcon.Open(dbURI), &gorm.Config{})
	return db, err
}

// 入力された商品が既に登録されていればそのIDを返す
func checkItem(db *gorm.DB, name string) (Stock, error) {
	var item Stock
	err := db.Where("name = ?", name).First(&item).Error
	return item, err
}

func insertNewItem(db *gorm.DB, name string, amount int) error {
	var insertData Stock
	insertData.Name = name
	insertData.Amount = amount
	jst, _ := time.LoadLocation("Asia/Tokyo")
	insertData.Created_at = time.Now().In(jst)
	insertData.Updated_at = time.Now().In(jst)

	err := db.Create(&insertData).Error
	return err
}

func updateItem(db *gorm.DB, id int, amount int) error {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	now := time.Now().In(jst)
	err := db.Model(Stock{}).Where("id = ?", id).Updates(Stock{Amount: amount, Updated_at: now}).Error
	return err
}
