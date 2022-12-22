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
		newrelic.ConfigLicense("NEW_RELIC_LICENSE_KEY"),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		panic(err)
	}

	log.Print("Connected to newrelic...")

	r := chi.NewRouter()

	r.Use(newrelicMiddleware(app))

	r.Post("/products/", Create)
	r.Get("/products/", GetAll)
	r.Get("/products/{id}", Get)
	r.Patch("/products/{id}", Update)
	r.Delete("/products/{id}", Delete)

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

// @desc: Create a Product
// @route: POST /products

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

// @desc: Get All Products
// @route: GET /products

func GetAll(w http.ResponseWriter, r *http.Request) {
	tracedDB := r.Context().Value("tracedDB").(*gorm.DB)
	var products []Product
	tracedDB.Find(&products)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(products)
}

// @desc: Get a Product
// @route: GET /products/:id

func Get(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

// @desc: Update a Product
// @route: PATCH /products/:id

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

// @desc: Delete a Product
// @route: DELETE /products/:id

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

// middleware to add newrelic transaction to the context
func newrelicMiddleware(app *newrelic.Application) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			gormTransactionTrace := app.StartTransaction(r.Method + " " + r.RequestURI)
			defer gormTransactionTrace.End()
			gormTransactionContext := newrelic.NewContext(ctx, gormTransactionTrace)
			tracedDB := database.WithContext(gormTransactionContext)
			ctx = context.WithValue(ctx, "tracedDB", tracedDB)
			w = gormTransactionTrace.SetWebResponse(w)
			gormTransactionTrace.SetWebRequestHTTP(r)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
