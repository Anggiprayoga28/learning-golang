package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

//go:embed templates/*
var templatesFS embed.FS

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

var db *sql.DB

func main() {
	var psqlInfo string

	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		psqlInfo = databaseURL
	} else {
		host := getEnv("DB_HOST", "learning-postgres")
		port := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USER", "postgres")
		password := getEnv("DB_PASSWORD", "mysecretpassword")
		dbname := getEnv("DB_NAME", "learningdb")
		psqlInfo = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, password, dbname)
	}

	var err error
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Test the connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Successfully connected to the database!")

	// Initialize Gin router
	r := gin.Default()

	// Define routes
	r.GET("/", func(c *gin.Context) {
		htmlContent, err := templatesFS.ReadFile("templates/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load template: %v", err)
			return
		}
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, string(htmlContent))
	})

	r.GET("/users", func(c *gin.Context) {
		// Fetch all users and return as JSON
		users, err := getUsersFromDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, users)
	})

	r.POST("/users", createUser)
	r.PUT("/users/:id", updateUser)
	r.DELETE("/users/:id", deleteUser)

	port := getEnv("PORT", "8086")
	r.Run(":" + port)
}

// Handler to create a new user
func createUser(c *gin.Context) {
	var user struct {
		Name       string `json:"name"`
		Department string `json:"department"`
		Email      string `json:"email"`
	}
	if err := c.ShouldBindJSON(&user); err != nil {
		log.Printf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID int
	err := db.QueryRow(`INSERT INTO users (name, department, email) VALUES ($1, $2, $3) RETURNING id`,
		user.Name, user.Department, user.Email).Scan(&userID)
	if err != nil {
		log.Printf("Failed to insert user into database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": userID})
}

// Handler to update a user
func updateUser(c *gin.Context) {
	id := c.Param("id")
	var user struct {
		Name       string `json:"name"`
		Department string `json:"department"`
		Email      string `json:"email"`
	}
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.Exec("UPDATE users SET name = $1, department = $2, email = $3 WHERE id = $4",
		user.Name, user.Department, user.Email, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated"})
}

// Handler to delete a user
func deleteUser(c *gin.Context) {
	id := c.Param("id")
	_, err := db.Exec("DELETE FROM users WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

// Fetch all users from the database
func getUsersFromDB() ([]map[string]interface{}, error) {
	rows, err := db.Query("SELECT id, name, department, email FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id int
		var name, department, email string
		err = rows.Scan(&id, &name, &department, &email)
		if err != nil {
			return nil, err
		}
		users = append(users, gin.H{"id": id, "name": name, "department": department, "email": email})
	}

	return users, nil
}
