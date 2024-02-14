package main

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionResponse struct {
	Limit   int `json:"limite"`
	Balance int `json:"saldo"`
}

type TransactionInput struct {
	// Value in cents
	Value json.Number `json:"valor"`
	// Type: "c" = + credito | "d" = - debito
	Type        string `json:"tipo"`
	// Description length 1..10
	Description string `json:"descricao"`
}

type StatementBalance struct {
	// Total in cents
	Total int    `json:"total"`
	Date  string `json:"data_extrato"`
	Limit int    `json:"limite"`
}

type StatementLastTransaction struct {
	// Value in cents
	Value int `json:"valor"`
	// Type: "c" = credito | "d" = debito
	Type        string `json:"tipo"`
	// Description length 1..10
	Description string `json:"descricao"`
	MadeAt      time.Time `json:"realizada_em"`
}

type StatementResponse struct {
	Balance          StatementBalance           `json:"saldo"`
	LastTransactions []StatementLastTransaction `json:"ultimas_transacoes"`
}

type RinhaHandler struct {
	db *pgxpool.Pool
}

func NewRinhaHandler(db *pgxpool.Pool) *RinhaHandler {
	return &RinhaHandler{
		db: db,
	}
}

