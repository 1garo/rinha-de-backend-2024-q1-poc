package main

import (
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
func transaction(w http.ResponseWriter, r *http.Request) {
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

	var input TransactionInput
	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}


	var resp TransactionResponse
	resp.Balance = 10
	resp.Limit = 10
	if response, err = json.Marshal(resp); err != nil {
		log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
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
func statement(w http.ResponseWriter, r *http.Request) {
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
	m := mux.NewRouter()
	m.HandleFunc("/clientes/{id}/extrato", statement).Methods("GET")
	m.HandleFunc("/clientes/{id}/transacoes", transaction).Methods("POST")

	http.Handle("/", m)
	err := http.ListenAndServe(":3333", nil)
	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}
