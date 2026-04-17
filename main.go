package main

import (
	"log"
	"net/http"

	"github.com/irwanrusda/rtmp-general-backend/app/config"
	"github.com/irwanrusda/rtmp-general-backend/app/routes"
)

func main() {
	// Initialize Database
	config.InitDB()
	defer config.DB.Close()

	log.Println("Skipping automatic scheme patching on startup. Run /api/migrate manually.")

	// Initialize Routes
	router := routes.InitRouter()

	// Apply Middlewares
	handler := routes.Logger(router)

	// Start Server on 8080 (nginx reverse-proxies /api/* to here)
	port := "8080"
	log.Printf("Golang RTMP API Backend started on :%s\n", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal("Server failed: ", err)
	}
}
