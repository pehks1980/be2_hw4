package main

import (
	"context"
	"database/sql"
	"encoding/json"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/negroni"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var (
	db *sql.DB
	measurable = MeasurableHandler
	port = "9090"
	shutdownTimeout = 5
)

// префикс перед лейблами
const (
	Namespace   = "metrics"
	LabelMethod = "method"
	LabelStatus = "status"
	LabelHandler = "handler"
)

var duration = prometheus.NewSummaryVec( prometheus.SummaryOpts {
		Namespace: Namespace,
		Name: "duration_seconds",
		Help: "Summary of request duration in seconds",
		Objectives: map[float64]float64{0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	},
	[]string{LabelHandler, LabelMethod, LabelStatus})

var errorsTotal = prometheus.NewCounterVec( prometheus.CounterOpts{
	Namespace: Namespace,
	Name: "errors_total",
	Help: "Total number of errors"},
	[]string{LabelHandler, LabelMethod, LabelStatus} )

var requestsTotal = prometheus.NewCounterVec( prometheus.CounterOpts{
	Namespace: Namespace,
	Name: "request_total",
	Help: "Total number of requests" },
	[]string{LabelHandler, LabelMethod} )

var MeasurableHandler = func(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		t := time.Now()
		m := r.Method
		p := r.URL.Path

		requestsTotal.WithLabelValues(p, m).Inc()

		mw := negroni.NewResponseWriter(w)
		//mw := &measurableWriter{w: w}
		h(mw, r)
		responseStatus := mw.Status()
		if  responseStatus/100 > 3 {
			errorsTotal.WithLabelValues(p, m, strconv.Itoa(responseStatus)).Inc()
		}

		duration.WithLabelValues(p, m, strconv.Itoa(responseStatus)).Observe(float64(time.Since(t).Microseconds()))
		log.Printf("calling %s duration %.2f", r.URL, float64(time.Since(t).Microseconds()) )
	}
}

func RegisterPublicHTTP() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/entities", measurable(ListEntitiesHandler)).Methods(http.MethodGet)
	r.HandleFunc("/entity", measurable(AddEntityHandler)).Methods(http.MethodPost)
	r.Handle("/metrics", promhttp.Handler())
	return r
}
const DDL=`CREATE TABLE IF NOT EXISTS entities (
		id INT PRIMARY KEY,
		data VARCHAR(32)
		);`
func main() {

	var err error
	db, err = sql.Open("mysql", "root:test@tcp(192.168.1.204:3306)/test")
	if err != nil {
		log.Fatalf("SQL problema: %v",err)
	}

	_, err = db.Exec(DDL)
	if err != nil {
		log.Fatalf("SQL DDL problema: %v",err)
	}

	defer db.Close()

	prometheus.MustRegister(duration,errorsTotal,requestsTotal)

	serv := http.Server{
		Addr:    net.JoinHostPort("", port),
		Handler: RegisterPublicHTTP(),
	}
	// запуск сервера
	go func() {
		if err := serv.ListenAndServe(); err != nil {
			log.Fatalf("listen and serve err: %v", err)
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	log.Printf("Started app at port = %s", port)
	// ждет сигнала
	sig := <-interrupt

	log.Printf("Sig: %v, stopping app", sig)
	// шат даун по контексту с тайм аутом
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownTimeout)*time.Second)
	defer cancel()
	if err := serv.Shutdown(ctx); err != nil {
		log.Printf("shutdown err: %v", err)
	}
}

const sqlInsertEntity = `INSERT INTO entities(id, data) VALUES (?, ?) `

func AddEntityHandler(w http.ResponseWriter, r *http.Request) {
 /*
	res, err := http.Get(fmt.Sprintf("http://acl/identity?token=%s",
	r.FormValue("token")))
	switch {
		case err != nil:
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	case res.StatusCode != http.StatusOK:
		w.WriteHeader(http.StatusForbidden)
		return
	}
	res.Body.Close()
*/
	_, err := db.Exec(sqlInsertEntity, r.FormValue("id"), r.FormValue("data"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(200)
}

const sqlSelectEntities = ` SELECT id, data FROM entities `

type ListEntityItemResponse struct {
	Id 		string `json:"id"`
	Data 	string `json:"data"`
}

func ListEntitiesHandler(w http.ResponseWriter, r *http.Request) {
	rr, err := db.Query(sqlSelectEntities)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rr.Close()

	var ii = []*ListEntityItemResponse{}
	for rr.Next() {
		i := &ListEntityItemResponse{}
		err = rr.Scan(&i.Id, &i.Data)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		ii = append(ii, i)
	}

	bb, err := json.Marshal(ii)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	_, err = w.Write(bb)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

/*
type measurableWriter struct {
	w 		http.ResponseWriter
	Status 	int
}

func (e measurableWriter) Header() http.Header {
	panic("implement me")
}

func (e measurableWriter) Write(p []byte) (int, error) {
	n, err := e.w.Write(p)
	if err != nil {
		return n, err
	}
	if n != len(p) {
		return n, io.ErrShortWrite
	}
	return len(p), nil
}

func (e measurableWriter) WriteHeader(statusCode int) {
	log.Printf("measurable writer code: %d",statusCode)
	e.Status = statusCode
	e.w.WriteHeader(statusCode)
}
*/