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
	"strconv"

	"cloud.google.com/go/cloudsqlconn"
	con "github.com/go-sql-driver/mysql"
	my "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type stock struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
}

func main() {
	log.Print("starting server...")
	http.HandleFunc("/hello", helloHandler)
	http.HandleFunc("/post", postHandler)
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

func postHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db, err := connectWithConnector()
	if err != nil {
		log.Println(err)
		http.Error(w, "Disable to connect db", http.StatusBadRequest)
		return
	}

	if err := createTable(db); err != nil {
		log.Println(err)
		http.Error(w, "Disable to create table", http.StatusBadRequest)
		return
	}

	if err := createTable(db); err != nil {
		log.Println(err)
		http.Error(w, "Disable to create table", http.StatusBadRequest)
		return
	}

	// リクエストボディからJSONデータを読み取り
	var request stock
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// JSONデータから名前を取得
	name := request.Name
	amount := request.Amount

	insert := `INSERT INTO stocks values(
		0,
		"` + name + `",
		` + strconv.Itoa(amount) + `,
		now(),
		now()
	);`

	err = db.Exec(insert).Error
	if err != nil {
		log.Println(err)
		http.Error(w, "Disable to insert data", http.StatusBadRequest)
		return
	}

	// レスポンスを生成
	response := fmt.Sprintf("name: %s\namount: %d", name, amount)

	// レスポンスをクライアントに返す
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "get from sql")
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

	// dbPool, err := sql.Open("mysql", dbURI)
	// if err != nil {
	// 	return nil, fmt.Errorf("sql.Open: %w", err)
	// }
	db, err := gorm.Open(my.Open(dbURI), &gorm.Config{})
	return db, err
}

//テーブル作成クエリを実行
func createTable(db *gorm.DB) error {
	createTable := `CREATE TABLE IF NOT EXISTS stocks (
		id INT NOT NULL ,
		name VARCHAR(8) NOT NULL,
		amount INT NOT NULL,
		created_at datetime NOT NULL,
		updated_at datetime NOT NULL,
		PRIMARY KEY (id)
	);`
	err := db.Exec(createTable).Error
	return err
}

//
// func checkItem(db *sql.DB) error {

// }
