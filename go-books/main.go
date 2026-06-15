package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type Book struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Genre  string `json:"genre"`
	Year   int    `json:"year"`
}

func JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		contentType := req.Header.Get("Content-Type")
		if req.Method == http.MethodPost && !strings.Contains(contentType, "application/json") {
			wr.WriteHeader(http.StatusUnsupportedMediaType)
			json.NewEncoder(wr).Encode(map[string]string{"error": "Content-Type must be application/json"})
			return
		}
		wr.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(wr, req)
	})
}

func getBooks(db *sql.DB) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		rows, err := db.Query("SELECT id, title, author, genre, year FROM books")
		if err != nil {
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var books []Book
		for rows.Next() {
			var b Book
			rows.Scan(&b.ID, &b.Title, &b.Author, &b.Genre, &b.Year)
			books = append(books, b)
		}
		json.NewEncoder(wr).Encode(books)
	}
}

func addBook(db *sql.DB) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		var b Book
		if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
			wr.WriteHeader(http.StatusBadRequest)
			return
		}

		err := db.QueryRow(
			"INSERT INTO books (title, author, genre, year) VALUES ($1, $2, $3, $4) RETURNING id",
			b.Title, b.Author, b.Genre, b.Year,
		).Scan(&b.ID)

		if err != nil {
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		wr.WriteHeader(http.StatusCreated)
		json.NewEncoder(wr).Encode(b)
	}
}

func deleteBook(db *sql.DB) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		id := vars["id"] // Беремо ID з URL

		_, err := db.Exec("DELETE FROM books WHERE id = $1", id)
		if err != nil {
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		wr.WriteHeader(http.StatusOK)
		json.NewEncoder(wr).Encode(map[string]string{"message": "Книгу видалено!"})
	}
}

func updateBook(db *sql.DB) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		id := vars["id"]

		var b Book
		if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
			wr.WriteHeader(http.StatusBadRequest)
			return
		}

		_, err := db.Exec(
			"UPDATE books SET title=$1, author=$2, genre=$3, year=$4 WHERE id=$5",
			b.Title, b.Author, b.Genre, b.Year, id,
		)

		if err != nil {
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		wr.WriteHeader(http.StatusOK)
		json.NewEncoder(wr).Encode(map[string]string{"message": "Книгу оновлено!"})
	}
}

func main() {
	dbURI := "postgres://postgres:mysecretpassword@127.0.0.1:5433/postgres?sslmode=disable"
	db, err := sql.Open("postgres", dbURI)
	if err != nil {
		log.Fatalln("DB Connection Error:", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalln("DB Ping Error:", err)
	}
	log.Println("Connected to PostgreSQL!")

	r := mux.NewRouter()
	r.Use(JSONMiddleware)

	r.HandleFunc("/books", getBooks(db)).Methods("GET")
	r.HandleFunc("/books", addBook(db)).Methods("POST")

	r.HandleFunc("/books/{id}", deleteBook(db)).Methods("DELETE")
	r.HandleFunc("/books/{id}", updateBook(db)).Methods("PUT", "PATCH")

	go func() {
		log.Println("Books API running on http://localhost:8080")
		http.ListenAndServe(":8080", r)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan
}
