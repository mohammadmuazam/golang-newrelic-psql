package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"database/sql"

	"github.com/go-chi/chi"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpgx"

	"github.com/newrelic/go-agent/v3/newrelic"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Product struct {
	gorm.Model
	Code  string
	Price uint
	Id    int
}

var database *gorm.DB

type resposne struct {
	Message string `json:"message"`
}

func main() {
	ConnectPostgres()

	log.Print("Starting server...")

	log.Print("Creating a newrelic application...")

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("batman"),
		newrelic.ConfigLicense("YOUR_LICENSE_KEY"),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		panic(err)
	}

	log.Print("Connected to newrelic...")

	r := chi.NewRouter()

	r.Use(newrelicMiddleware(app))

	r.Post("/", Create)
	r.Get("/", Get)
	r.Patch("/{id}", Update)
	r.Delete("/{id}", Delete)

	log.Print("Server started on port 3000")
	http.ListenAndServe(":3000", r)
}

func ConnectPostgres() {
	dsn := "host=localhost user=postgres password=root dbname=test port=5432 sslmode=disable TimeZone=Asia/Kolkata"
	conn, err := sql.Open("nrpgx", dsn)

	if err != nil {
		log.Fatalf("error to open connection with database %s", err.Error())
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: conn,
	}), &gorm.Config{})

	dbSql, err := db.DB()

	dbSql.SetMaxOpenConns(10)
	dbSql.SetMaxIdleConns(10)
	dbSql.SetConnMaxLifetime(time.Second * time.Duration(10))

	if err != nil {
		log.Fatalf("Fail to initialize database %s", err.Error())
	}

	db.AutoMigrate(&Product{})

	database = db
}

func Create(w http.ResponseWriter, r *http.Request) {
	tracedDB := r.Context().Value("tracedDB").(*gorm.DB)

	var product Product
	json.NewDecoder(r.Body).Decode(&product)
	if product.Code == "" || product.Price == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resposne{
			Message: "Code and Price are required",
		})
		return
	}

	var count int64
	tracedDB.Model(&Product{}).Where("code = ?", product.Code).Count(&count)
	if count > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resposne{
			Message: "Product already exists",
		})
		return
	}
	tracedDB.Create(&product)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resposne{
		Message: "Product created successfully",
	})
}

func Get(w http.ResponseWriter, r *http.Request) {
	tracedDB := r.Context().Value("tracedDB").(*gorm.DB)
	var products []Product
	tracedDB.Find(&products)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(products)
}

func Update(w http.ResponseWriter, r *http.Request) {
	tracedDB := r.Context().Value("tracedDB").(*gorm.DB)
	paramId := chi.URLParam(r, "id")
	var product Product
	id, _ := strconv.Atoi(paramId)
	tracedDB.First(&product, &Product{
		Id: id,
	})
	if product.Id == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resposne{
			Message: "Product not found",
		})
		return
	}
	json.NewDecoder(r.Body).Decode(&product)
	var count int64
	tracedDB.Model(&Product{}).Where("code = ?", product.Code).Count(&count)
	if count > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resposne{
			Message: "Product with this id already exists",
		})
		return
	}
	tracedDB.Save(&product)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resposne{
		Message: "Product updated successfully",
	})
}

func Delete(w http.ResponseWriter, r *http.Request) {
	tracedDB := r.Context().Value("tracedDB").(*gorm.DB)
	paramId := chi.URLParam(r, "id")
	var product Product
	id, _ := strconv.Atoi(paramId)
	tracedDB.First(&product, &Product{
		Id: id,
	})
	if product.Id == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resposne{
			Message: "Product not found",
		})
		return
	}
	tracedDB.Delete(&product)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resposne{
		Message: "Product deleted successfully",
	})
}

// middleware
func newrelicMiddleware(app *newrelic.Application) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			gormTransactionTrace := app.StartTransaction(r.Method + " " + r.RequestURI)
			defer gormTransactionTrace.End()
			gormTransactionContext := newrelic.NewContext(ctx, gormTransactionTrace)
			tracedDB := database.WithContext(gormTransactionContext)
			ctx = context.WithValue(ctx, "tracedDB", tracedDB)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
