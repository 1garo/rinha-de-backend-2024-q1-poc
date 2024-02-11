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

	"github.com/gorilla/mux"
)

type RinhaHandler struct {
	db *sql.DB
}

func NewRinhaHandler(db *sql.DB) *RinhaHandler {
	return &RinhaHandler{
		db: db,
	}
}

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

	var input TransactionInput
	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if input.Type == "d" {
		// TODO: add value to balance
		// Query:
		/*
			UPDATE "saldos"
			SET valor = valor - 100000
			from (select limite from "clientes" WHERE clientes.id = 1)
			where cliente_id = 1
			  AND abs(saldos.valor - 100000) <= limite;
		*/
		value := input.Value * 100
		result, err := rh.db.Exec(`
			UPDATE "saldos"
			SET valor = valor - ?
			from (select limite from "clientes" WHERE clientes.id = ?)
			where cliente_id = ?
			  AND abs(saldos.valor - ?) <= limite;
			  `, value, id, id, value)

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			log.Fatalln(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(response)
			return
		}

		if rowsAffected == 0 {
			log.Println("[ERROR]: 0 rows affected on UPDATE.")

			resp := make(map[string]string)
			resp["error"] = "Unprocessable Content"

			if response, err = json.Marshal(resp); err != nil {
				log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
			}

			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(response)
			return
		}

	} else if input.Type == "c" {
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
	db := r.Context().Value("db")

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
	connStr := "postgresql://admin:123@localhost/rinha"
	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
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
}
