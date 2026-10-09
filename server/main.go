package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"electricredit/database"
	appinternal "electricredit/internal"
)

func main() {
	if err := loadEnvFile(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("warning: unable to load .env: %v", err)
	}

	initDB := flag.Bool(
		"init-db",
		false,
		"initialize or safely update the ElectriCredit database",
	)

	createSuperuser := flag.String(
		"create-superuser",
		"",
		"create an authorized account: owner, admin, or developer",
	)

	resetPassword := flag.Bool(
		"reset-password",
		false,
		"reset an existing owner, admin, or developer password by email",
	)

	email := flag.String("email", "", "email for --create-superuser")
	password := flag.String("password", "", "password for --create-superuser")
	firstName := flag.String("first-name", "", "first name for --create-superuser")
	lastName := flag.String("last-name", "", "last name for --create-superuser")

	flag.Parse()

	db, err := database.Open("")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	/*
	 * Always run the idempotent initializer so safe schema updates are
	 * automatically applied to an existing database.
	 */
	if err := database.CreateDatabase(db, "database/price.json"); err != nil {
		log.Fatal(err)
	}

	if *initDB {
		fmt.Println("ElectriCredit database initialized successfully.")
		return
	}

	if *resetPassword {
		if err := appinternal.ResetSuperuserPassword(db, *email, *password); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("ElectriCredit password updated successfully for %s.\n", *email)
		return
	}

	if *createSuperuser != "" {
		if err := appinternal.CreateSuperuser(
			db,
			*createSuperuser,
			*firstName,
			*lastName,
			*email,
			*password,
		); err != nil {
			log.Fatal(err)
		}

		fmt.Printf(
			"ElectriCredit %s account created successfully for %s.\n",
			*createSuperuser,
			*email,
		)
		return
	}

	mux := http.NewServeMux()
	appinternal.RegisterRoutes(mux, db)

	clientDir := os.Getenv("ELECTRICREDIT_CLIENT_DIR")

	if clientDir == "" {
		clientDir = filepath.Join("..", "client")
	}

	absoluteClientDir, err := filepath.Abs(clientDir)
	if err != nil {
		log.Fatal(err)
	}

	staticFiles := http.FileServer(http.Dir(absoluteClientDir))
	mux.Handle("/", withStaticCache(staticFiles))

	address := os.Getenv("ELECTRICREDIT_ADDR")
	if address == "" {
		address = ":8080"
	}

	displayAddress := address
	if address == ":8080" {
		displayAddress = "127.0.0.1:8080"
	}
	fmt.Printf("ElectriCredit is available at http://%s\n", displayAddress)

	server := &http.Server{
		Addr:    address,
		Handler: appinternal.WithLocalDevCORS(mux),
	}

	serverErr := make(chan error, 1)

	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErr <- err
	}()

	shutdownSignal, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Printf("ElectriCredit server stopped unexpectedly: %v", err)
		}

	case <-shutdownSignal.Done():
		fmt.Println("\nStopping ElectriCredit...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown timed out: %v", err)
			_ = server.Close()
		}

		if err := <-serverErr; err != nil {
			log.Printf("ElectriCredit server stopped with an error: %v", err)
		}

		fmt.Println("ElectriCredit stopped cleanly.")
	}
}

func withStaticCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(filepath.Ext(r.URL.Path))

		switch ext {
		case ".html", "":
			w.Header().Set("Cache-Control", "no-cache")
		case ".css", ".js":
			if r.URL.Query().Get("v") != "" {
				w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
			} else {
				w.Header().Set("Cache-Control", "public, max-age=86400")
			}
		case ".png", ".jpg", ".jpeg", ".svg", ".webp", ".ico", ".ino":
			w.Header().Set("Cache-Control", "public, max-age=604800")
		}

		next.ServeHTTP(w, r)
	})
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}

		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}
