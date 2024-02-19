package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
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
func (rh *RinhaHandler) transaction(ctx *fasthttp.RequestCtx) {
	var err error
	var response []byte

	ctx.SetContentType("application/json")

	var input TransactionInput
	err = json.Unmarshal(ctx.PostBody(), &input)
	if err != nil {
		log.Printf("[ERROR]: In JSON unmarshal. Err: %s\n", err)

		resp := make(map[string]string)
		resp["error"] = "Internal Server Error"
		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.Write(response)
		return
	}

	value, err := input.Value.Int64()
	if value == 0 {
		resp := make(map[string]string)
		resp["error"] = "StatusUnprocessableEntity"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}

		ctx.SetStatusCode(http.StatusUnprocessableEntity)
		ctx.Write(response)
		return

	}

	if len(input.Description) < 1 || len(input.Description) > 10 {
		resp := make(map[string]string)
		resp["error"] = "StatusUnprocessableEntity"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}

		ctx.SetStatusCode(http.StatusUnprocessableEntity)
		ctx.Write(response)
		return
	}

	idStr := ctx.UserValue("id")
	id, err := strconv.Atoi(idStr.(string))
	if err != nil {
		log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)

		resp := make(map[string]string)
		resp["error"] = "Internal Server Error"
		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.Write(response)
		return
	}

	// TODO: put this validation on a function because its used twice
	var clienteIDExists bool
	err = rh.db.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM clientes WHERE id = $1)", id).Scan(&clienteIDExists)
	if err != nil {
		log.Fatal(err)
	}

	if !clienteIDExists {
		resp := make(map[string]string)
		resp["error"] = "Not Found"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		ctx.SetStatusCode(http.StatusNotFound)
		ctx.Write(response)
		return
	}


	if input.Type == "d" {
		// TODO: remove value from balance
		var limite, saldo int
		err = rh.db.QueryRow(context.Background(), `
			UPDATE saldos
			SET valor = valor - $1
			FROM (SELECT limite FROM clientes WHERE id = $2) AS cliente_limite
			WHERE cliente_id = $3
			  AND abs(saldos.valor - $4) <= cliente_limite.limite
			RETURNING limite, valor;
		`, value, id, id, value).Scan(&limite, &saldo)

		if errors.Is(err, pgx.ErrNoRows) {
			log.Println("[ERROR]: No rows.")

			resp := make(map[string]string)
			resp["error"] = "Unprocessable Entity"

			if response, err = json.Marshal(resp); err != nil {
				log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
			}

			ctx.SetStatusCode(http.StatusUnprocessableEntity)
			ctx.Write(response)
			return
		}

		if err != nil {
			log.Printf("query row: err %v\n", err)
			ctx.SetStatusCode(http.StatusInternalServerError)
			ctx.Write(response)
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
		var limite, saldo int
		err = rh.db.QueryRow(context.Background(), `
			UPDATE saldos
			SET valor = valor + $1
			FROM (SELECT limite FROM clientes WHERE id = $2)
			WHERE cliente_id = $3
			RETURNING limite, valor;
		`, value, id, id).Scan(&limite, &saldo)

		if errors.Is(err, pgx.ErrNoRows) {
			log.Println("[ERROR]: No rows.")

			resp := make(map[string]string)
			resp["error"] = "Unprocessable Entity"

			if response, err = json.Marshal(resp); err != nil {
				log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
			}

			ctx.SetStatusCode(http.StatusUnprocessableEntity)
			ctx.Write(response)
			return
		}

		if err != nil {
			resp := make(map[string]string)
			resp["error"] = "StatusUnprocessableEntity"

			if response, err = json.Marshal(resp); err != nil {
				log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
			}

			ctx.SetStatusCode(http.StatusUnprocessableEntity)
			ctx.Write(response)
			return
		}

		var resp TransactionResponse
		resp.Limit = limite
		resp.Balance = saldo

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
	} else {
		resp := make(map[string]string)
		resp["error"] = "unprocessable entity"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}

		ctx.SetStatusCode(http.StatusUnprocessableEntity)
		ctx.Write(response)
		return
	}

	_, err = rh.db.Exec(context.Background(),
		"insert into \"transacoes\"(cliente_id, valor, tipo, descricao) values($1, $2, $3, $4)",
		id, value, input.Type, input.Description,
	)

	if err != nil {
		log.Printf("query row: err %v\n", err)
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.Write(response)
		return
	}

	ctx.SetStatusCode(http.StatusOK)
	ctx.Write(response)
	return
}

/*
Regras Se o atributo [id] da URL for de uma identificação não existente de cliente, a API deve retornar HTTP Status Code 404.
O corpo da resposta nesse caso não será testado e você pode escolher como o representar.
Já sabe o que acontece se sua API retornar algo na faixa 2XX, né? Agradecido.
*/
func (rh *RinhaHandler) statement(ctx *fasthttp.RequestCtx) {
	fmt.Println("[REQUEST]: statement")
	var err error
	var response []byte

	ctx.SetContentType("application/json")

	idStr := ctx.UserValue("id")
	id, err := strconv.Atoi(idStr.(string))
	if err != nil {
		log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)

		resp := make(map[string]string)
		resp["error"] = "Internal Server Error"
		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.Write(response)
		return
	}

	// TODO: put this validation on a function because its used twice
	var clienteIDExists bool
	err = rh.db.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM clientes WHERE id = $1)", id).Scan(&clienteIDExists)
	if err != nil {
		log.Fatal("Erro aqui: ", err)
	}

	if !clienteIDExists {
		resp := make(map[string]string)
		resp["error"] = "Not Found"

		if response, err = json.Marshal(resp); err != nil {
			log.Printf("[ERROR]: In JSON marshal. Err: %s\n", err)
		}
		ctx.SetStatusCode(http.StatusNotFound)
		ctx.Write(response)
		return
	}

	rows, err := rh.db.Query(context.Background(), `
		SELECT transacoes.valor, transacoes.realizada_em, transacoes.tipo, transacoes.descricao,
			   saldos.valor as total, clientes.limite
		FROM "clientes"
		JOIN "saldos" ON saldos.cliente_id = clientes.id
		LEFT JOIN "transacoes" ON transacoes.cliente_id = clientes.id  -- Use LEFT JOIN here

		WHERE clientes.id = $1  -- Add conditions for the specific client
		ORDER BY transacoes.realizada_em DESC
		LIMIT 10
		`, id)
	defer rows.Close()

	var statementResponse StatementResponse

	for rows.Next() {
		var valueScan, totalScan, limiteScan sql.NullInt64
		var tipoScan, descricaoScan sql.NullString
		var realizadaEmScan sql.NullTime

		// Scan values from the row into variables
		err := rows.Scan(&valueScan, &realizadaEmScan, &tipoScan, &descricaoScan, &totalScan, &limiteScan)
		if err != nil {
			log.Fatal(err)
			return
			// return statementResponse, err
		}

		// Populate the StatementBalance struct
		statementResponse.Balance = StatementBalance{
			// You may need to format the date based on your specific requirements
			Date: time.Now().UTC().Format(time.RFC3339),
		}

		if limiteScan.Valid {
			statementResponse.Balance.Limit = int(limiteScan.Int64)
		}

		if totalScan.Valid {
			statementResponse.Balance.Total = int(totalScan.Int64)
		}

		if realizadaEmScan.Valid && valueScan.Valid && tipoScan.Valid && descricaoScan.Valid {
			// Populate the StatementLastTransaction slice
			statementResponse.LastTransactions = append(statementResponse.LastTransactions, StatementLastTransaction{
				Value:       int(valueScan.Int64),
				Type:        string(tipoScan.String),
				Description: string(descricaoScan.String),
				MadeAt:      time.Time(realizadaEmScan.Time),
			})
		}
	}

	if response, err = json.Marshal(statementResponse); err != nil {
		log.Printf("Error happened in JSON marshal. Err: %s", err)
	}

	ctx.SetStatusCode(http.StatusOK)
	ctx.Write(response)
	return
}

func main() {
	connStr := "postgresql://admin:123@db:5432/rinha?sslmode=disable"
	// Connect to database
	dbpool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	// Create the Store and Recipe Handler
	handler := NewRinhaHandler(dbpool)

	r := router.New()
	r.GET("/clientes/{id}/extrato", handler.statement)
	r.POST("/clientes/{id}/transacoes", handler.transaction)

	fmt.Println("listening on port 3000")
	err = fasthttp.ListenAndServe(":3000", r.Handler)
	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}
