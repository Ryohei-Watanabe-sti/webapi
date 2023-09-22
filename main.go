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
	con "github.com/go-sql-driver/mysql"
	my "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Stocks struct {
	gorm.Model
	Id         int       `json:"-" gorm:"primaryKey,autoIncrement,not null"`
	Name       string    `json:"name" gorm:"not null"`
	Amount     int       `json:"amount" gorm:"not null"`
	Created_at time.Time `json:"-" gorm:"not null"`
	Updated_at time.Time `json:"-" gorm:"not null"`
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

	// リクエストボディからJSONデータを読み取り
	var request Stocks
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		log.Println(err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	//テーブル存在チェック
	if db.Migrator().HasTable(&Stocks{}) == false {
		//テーブル作成クエリを実行
		if err := db.Migrator().CreateTable(&Stocks{}).Error; err != nil {
			log.Println(err())
			http.Error(w, "Fail to create table", http.StatusInternalServerError)
			return
		}
	}

	// JSONデータから名前を取得
	name := request.Name
	amount := request.Amount

	oldStock, err := checkItem(db, name)

	if strings.Contains(err.Error(), "record not found") {
		log.Println("New Item arrival!")
	} else if err != nil {
		log.Println(err)
		http.Error(w, "Fail to check table", http.StatusInternalServerError)
		return
	}

	if strings.Contains(err.Error(), "record not found") {
		if err := insertNewItem(db, name, amount); err != nil {
			log.Println(err)
			http.Error(w, "Fail to insert new item", http.StatusInternalServerError)
		}
	} else {
		amount = amount + oldStock.Amount
		if err := plusItem(db, oldStock, amount); err != nil {
			log.Println(err)
			http.Error(w, "Fail to update new item", http.StatusInternalServerError)
		}
	}

	// レスポンスを生成
	response := fmt.Sprintf("name: %s\namount: %d", name, amount)

	// レスポンスをクライアントに返す
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	var stocks []Stocks
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
	result := db.Find(&stocks)
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
	// Note: Saving credentials in environment variables is convenient, but not
	// secure - consider a more secure solution such as
	// Cloud Secret Manager (https://cloud.google.com/secret-manager) to help
	// keep passwords and other secrets safe.
	var (
		dbUser                 = mustGetenv("DB_USER")                  // e.g. 'my-db-user'
		dbPwd                  = mustGetenv("DB_PASS")                  // e.g. 'my-db-password'
		dbName                 = mustGetenv("DB_NAME")                  // e.g. 'my-database'
		instanceConnectionName = mustGetenv("INSTANCE_CONNECTION_NAME") // e.g. 'project:region:instance'
		usePrivate             = os.Getenv("PRIVATE_IP")
	)

	d, err := cloudsqlconn.NewDialer(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cloudsqlconn.NewDialer: %w", err)
	}
	var opts []cloudsqlconn.DialOption
	if usePrivate != "" {
		opts = append(opts, cloudsqlconn.WithPrivateIP())
	}
	con.RegisterDialContext("cloudsqlconn",
		func(ctx context.Context, addr string) (net.Conn, error) {
			return d.Dial(ctx, instanceConnectionName, opts...)
		})
	dbURI := fmt.Sprintf("%s:%s@cloudsqlconn(localhost:3306)/%s?parseTime=true&loc=Asia%%2FTokyo",
		dbUser, dbPwd, dbName)

	db, err := gorm.Open(my.Open(dbURI), &gorm.Config{})
	return db, err
}

//入力された商品が既に登録されていればそのIDを返す
func checkItem(db *gorm.DB, name string) (Stocks, error) {
	var item Stocks
	err := db.Where("name = ?", name).First(&item).Error
	return item, err
}

func insertNewItem(db *gorm.DB, name string, amount int) error {
	var insertData Stocks
	insertData.Name = name
	insertData.Amount = amount
	jst, _ := time.LoadLocation("Asia/Tokyo")
	insertData.Created_at = time.Now().In(jst)
	insertData.Updated_at = time.Now().In(jst)

	err := db.Create(&insertData).Error
	return err
}

func plusItem(db *gorm.DB, insertData Stocks, amount int) error {
	err := db.Model(&insertData).Update("amount", amount).Error
	return err
}

func shipmentHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "shipment function")
}
