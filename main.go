package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"

	"github.com/gorilla/mux"
)

/*
Regras Uma transação de débito nunca pode deixar o saldo do cliente menor que seu limite disponível.
Por exemplo, um cliente com limite de 1000 (R$ 10) nunca deverá ter o saldo menor que -1000 (R$ -10).
Nesse caso, um saldo de -1001 ou menor significa inconsistência na Rinha de Backend!

Se uma requisição para débito for deixar o saldo inconsistente, a API deve retornar HTTP Status Code 422 sem completar a transação!
O corpo da resposta nesse caso não será testado e você pode escolher como o representar.

Se o atributo [id] da URL for de uma identificação não existe de cliente, a API deve retornar HTTP Status Code 404.
O corpo da resposta nesse caso não será testado e você pode escolher como o representar.
Se a API retornar algo como HTTP 200 informando que o cliente não foi encontrado no corpo da resposta ou HTTP 204 sem corpo,
ficarei extremamente deprimido e a Rinha será cancelada para sempre.
*/
func (rh *RinhaHandler) transaction(w http.ResponseWriter, r *http.Request) {
	var err error
	var response []byte
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)

		resp := make(map[string]string)
		resp["error"] = "Bad Request"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(response)
		return
	}

	var clienteIDExists bool
	err = rh.db.QueryRow("SELECT EXISTS(SELECT 1 FROM clientes WHERE id = $1)", id).Scan(&clienteIDExists)
	if err != nil {
		log.Fatal(err)
	}

	if !clienteIDExists {
		resp := make(map[string]string)
		resp["error"] = "Not Found"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write(response)
		return
	}


	var input TransactionInput
	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if 1 < len(input.Description) && len(input.Description) > 10 {
		resp := make(map[string]string)
		resp["error"] = "Bad Request"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(response)
		return
	}

	if input.Type == "d" {
		// TODO: remove value from balance
		var limite, saldo int
		err = rh.db.QueryRow(`
			UPDATE saldos
			SET valor = valor - $1
			FROM (SELECT limite FROM clientes WHERE clientes.id = $2) AS cliente_limite
			WHERE cliente_id = $3
			  AND abs(saldos.valor - $4) <= cliente_limite.limite
			RETURNING limite, valor;
		`, input.Value, id, id, input.Value).Scan(&limite, &saldo)

		if errors.Is(err, sql.ErrNoRows) {
			log.Println("[ERROR]: No rows.")

			resp := make(map[string]string)
			resp["error"] = "Unprocessable Entity"

			if response, err = json.Marshal(resp); err != nil {
				log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
			}

			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(response)
			return
		}
		if err != nil {
			log.Fatalf("query row: err %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(response)
			return
		}

		var resp TransactionResponse
		resp.Limit = limite
		resp.Balance = saldo

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
	} else if input.Type == "c" {
		// TODO: add value to saldo
	} else {
		resp := make(map[string]string)
		resp["error"] = "Not supported type"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(response)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
	return
}

/*
Regras Se o atributo [id] da URL for de uma identificação não existente de cliente, a API deve retornar HTTP Status Code 404.
O corpo da resposta nesse caso não será testado e você pode escolher como o representar.
Já sabe o que acontece se sua API retornar algo na faixa 2XX, né? Agradecido.
*/
func (rh *RinhaHandler) statement(w http.ResponseWriter, r *http.Request) {
	fmt.Println("[REQUEST]: statement")
	var err error
	var response []byte

	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	_, err = strconv.Atoi(vars["id"])
	if err != nil {
		log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)

		resp := make(map[string]string)
		resp["error"] = "Bad Request"
		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(response)
		return
	}
	// db := r.Context().Value("db")

	resp := StatementResponse{
		Balance: StatementBalance{
			Total: -9098,
			Date:  time.Now().UTC().Format(time.RFC3339),
			Limit: 100000,
		},
		LastTransactions: []StatementLastTransaction{
			{
				Value:       10,
				Type:        "c",
				Description: "descricao",
				MadeAt:      time.Now().UTC().Format(time.RFC3339),
			},
			{
				Value:       10,
				Type:        "d",
				Description: "descricao",
				MadeAt:      time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	if response, err = json.Marshal(resp); err != nil {
		log.Printf("Error happened in JSON marshal. Err: %s", err)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
	return
}

func main() {
	connStr := "postgresql://admin:123@localhost:5432/rinha?sslmode=disable"
	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("could not open db: err %v", err)
	}

	// Create the Store and Recipe Handler
	handler := NewRinhaHandler(db)

	m := mux.NewRouter()
	m.HandleFunc("/clientes/{id}/extrato", handler.statement).Methods("GET")
	m.HandleFunc("/clientes/{id}/transacoes", handler.transaction).Methods("POST")

	http.Handle("/", m)
	err = http.ListenAndServe(":3000", nil)
	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("server started, listening on port 3000")
}
