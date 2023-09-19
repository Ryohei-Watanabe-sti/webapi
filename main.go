// Sample run-helloworld is a minimal Cloud Run service.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type stock struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
}

func main() {
	log.Print("starting server...")
	http.HandleFunc("/hello", helloHandler)
	http.HandleFunc("/post", postHandler)

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

	db := getDB()
	defer db.Close()

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

	// レスポンスを生成
	response := fmt.Sprintf("name: %s\n amount: %d", name, amount)

	// レスポンスをクライアントに返す
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func getDB() *sql.DB {
	// cleanup, err := pgxv4.RegisterDriver("cloudsql-mysql", cloudsqlconn.WithIAMAuthN())
	// if err != nil {
	// 	log.Fatalf("Error on pgxv4.RegisterDriver: %v", err)
	// }

	dsn := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable", os.Getenv("INSTANCE_CONNECTION_NAME"), os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	db, err := sql.Open("cloudsql-mysql", dsn)
	if err != nil {
		log.Fatalf("Error on sql.Open: %v", err)
	}

	createVisits := `CREATE TABLE IF NOT EXISTS visits (
	  id INT NOT NULL,
	  created_at timestamp NOT NULL,
	  PRIMARY KEY (id)
	);`
	_, err = db.Exec(createVisits)
	if err != nil {
		log.Fatalf("unable to create table: %s", err)
	}

	return db
}

func connectDB() *sql.DB {
	db, err := sql.Open("mysql", "user:password@tcp(127.0.0.1:3306)/dbname?parseTime=true&loc=Asia%2FTokyo")
	if err != nil {
		log.Println(err)
	}
	return db
}
