package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/getsentry/sentry-go"
	_ "github.com/go-sql-driver/mysql"
	"github.com/logrusorgru/aurora/v3"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
)

var global Config
var db *sql.DB
var salt []byte
var dataDogClient *statsd.Client

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse form for further usage.
		r.ParseForm()

		if global.Debug {
			log.Printf("%s %s", aurora.Blue(r.Method), aurora.Red(r.URL))
			for name, value := range r.Form {
				log.Print(name, " ", aurora.Green("=>"), " ", value)
			}

			log.Printf("Accessing from: %s", aurora.Blue(r.Host))
		}

		// Finally, serve.
		handler.ServeHTTP(w, r)
	})
}

func checkHandler(w http.ResponseWriter, r *http.Request) {
	Check(w, r, db, strconv.Itoa(global.Interval))
}

func receiveHandler(w http.ResponseWriter, r *http.Request) {
	Receive(w, r, db)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	Delete(w, r, db)
}

func sendHandler(w http.ResponseWriter, r *http.Request) {
	Send(w, r, db, global)
}

func configHandle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		r.ParseForm()

		fileWriter, _, err := r.FormFile("uploaded_config")
		if err != nil || err == http.ErrMissingFile {
			LogError("Incorrect file", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "It seems your file upload went awry. Contact our support email: %s\nError: %v", global.SupportEmail, err)
			return
		}

		file, err := ioutil.ReadAll(fileWriter)
		if err != nil {
			LogError("Unable to read file", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "It seems your file upload went awry. Contact our support email: %s\nError: %v", global.SupportEmail, err)
			return
		}

		patched, err := ModifyNwcConfig(file)
		if err != nil {
			LogError("Unable to patch", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "It seems your patching went awry. Contact our support email: %s\nError: %v", global.SupportEmail, err)
			return
		}
		w.Header().Add("Content-Type", "application/octet-stream")
		w.Header().Add("Content-Disposition", "attachment; filename=\"nwc24msg.cfg\"")
		w.Write(patched)
		break
	case "GET":
		fmt.Fprint(w, "This page doesn't do anything by itself. Try going to the main site.")
	default:
		break
	}
}

func main() {
	if global.Datadog {
		tracer.Start(
			tracer.WithService("mail"),
			tracer.WithEnv("prod"),
			tracer.WithAgentAddr("127.0.0.1:8126"),
		)
		defer tracer.Stop()

		if err := profiler.Start(
			profiler.WithService("mail"),
			profiler.WithEnv("prod"),
		); err != nil {
			log.Fatal(err)
		}
		defer profiler.Stop()
	}

	// Get salt for passwords
	saltLocation := "config/salt.bin"
	salt, err := ioutil.ReadFile(saltLocation)
	if os.IsNotExist(err) {
		log.Println("No salt found. Creating....")
		salt = make([]byte, 128)

		_, err := rand.Read(salt)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile("config/salt.bin", salt, os.ModePerm)
		if err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	}

	// Read config
	file, err := os.Open("config/config.json")
	if err != nil {
		panic(err)
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&global)
	if err != nil {
		panic(err)
	}

	if global.Debug {
		log.Println("Connecting to MySQL...")
	}
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		global.Username, global.Password, global.Host, global.Port, global.DBName))
	if err != nil {
		panic(err)
	}

	// Ensure Mail-Go does not overload the backing database.
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	// Prepare database
	initAccountDB()
	initAuthDB()
	initCheckDB()
	initDeleteDB()
	initInboundParseDB()
	initReceiveDB()
	initSendDB()

	// Configure Sentry
	if global.RavenDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn: global.RavenDSN,
		})
		if err != nil {
			panic(err)
		}
	}

	// Lastly, Datadog as a whole.
	if global.Datadog {
		dataDogClient, err = statsd.New("127.0.0.1:8125")
		if err != nil {
			panic(err)
		}
	}

	// Mail calls
	http.HandleFunc("/cgi-bin/account.cgi", Account)
	http.HandleFunc("/cgi-bin/patcher.cgi", Account)
	http.HandleFunc("/cgi-bin/check.cgi", checkHandler)
	http.HandleFunc("/cgi-bin/receive.cgi", receiveHandler)
	http.HandleFunc("/cgi-bin/delete.cgi", deleteHandler)
	http.HandleFunc("/cgi-bin/send.cgi", sendHandler)

	mailDomain = regexp.MustCompile(`w(\d{16})\@(` + global.SendGridDomain + `)`)

	// Inbound parse
	http.HandleFunc("/sendgrid/parse", sendGridHandler)

	// Site
	http.HandleFunc("/patch", configHandle)
	http.Handle("/", http.FileServer(http.Dir("./patch")))

	log.Println("Running...")

	// We do this to log all access to the page.
	log.Fatal(http.ListenAndServe(global.BindTo, logRequest(http.DefaultServeMux)))
}
