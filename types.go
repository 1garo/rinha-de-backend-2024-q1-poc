package main

type TransactionResponse struct {
	Limit   int `json:"limite"`
	Balance int `json:"saldo"`
}

type TransactionInput struct {
	// Value in cents
	Value int `json:"valor"`
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
	MadeAt      string `json:"realizada_em"`
}

type StatementResponse struct {
	Balance          StatementBalance           `json:"saldo"`
	LastTransactions []StatementLastTransaction `json:"ultimas_transacoes"`
}
