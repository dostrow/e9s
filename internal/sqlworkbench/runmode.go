package sqlworkbench

import (
	"fmt"
	"strings"
)

// RunMode chooses how a run treats transactions.
type RunMode int

const (
	// RunAsWritten sends the statements as they are; each commits as it
	// completes unless the SQL opens a transaction itself.
	RunAsWritten RunMode = iota
	// RunDryRun runs the statements inside a transaction that e9s always
	// rolls back, so nothing is kept.
	RunDryRun
	// RunInTransaction runs the statements inside a transaction that e9s
	// leaves open, so the results can be reviewed before committing.
	RunInTransaction
)

func (m RunMode) String() string {
	switch m {
	case RunDryRun:
		return "dry run"
	case RunInTransaction:
		return "run in a transaction"
	default:
		return "run"
	}
}

// transactionPlan is a script prepared to run inside e9s's transaction.
type transactionPlan struct {
	begin    string // the statement that opens the transaction
	body     []Statement
	replaced []string // the script's own transaction control e9s replaced
}

// planTransaction prepares statements to run inside one transaction that
// e9s opens and ends. A leading BEGIN is used to open it, keeping any
// isolation level it sets, and a final COMMIT or ROLLBACK is dropped: e9s
// decides how the transaction ends. Any other statement that would end the
// transaction early is refused, while savepoints are allowed.
func planTransaction(statements []Statement, source string) (transactionPlan, error) {
	plan := transactionPlan{begin: "BEGIN", body: statements}
	if len(plan.body) > 0 && startsTransaction(plan.body[0].SQL) {
		plan.begin = plan.body[0].SQL
		plan.replaced = append(plan.replaced, strings.ToUpper(SQLWords(plan.body[0].SQL)[0]))
		plan.body = plan.body[1:]
	}
	if last := len(plan.body) - 1; last >= 0 && endsTransaction(plan.body[last].SQL) {
		plan.replaced = append(plan.replaced, strings.ToUpper(SQLWords(plan.body[last].SQL)[0]))
		plan.body = plan.body[:last]
	}
	for _, statement := range plan.body {
		if IsTransactionControl(statement.SQL) && !isSavepointControl(statement.SQL) {
			return transactionPlan{}, fmt.Errorf("line %d: %s would end e9s's transaction partway through the script; remove it, or use Run to execute the script as written",
				StatementLine(source, statement.Start), strings.ToUpper(SQLWords(statement.SQL)[0]))
		}
	}
	if len(plan.body) == 0 {
		return transactionPlan{}, fmt.Errorf("there are no statements to run inside the transaction")
	}
	return plan, nil
}

func (p transactionPlan) event(mode RunMode) string {
	if len(p.replaced) == 0 {
		return ""
	}
	verb := "was"
	if len(p.replaced) > 1 {
		verb = "were"
	}
	return fmt.Sprintf("The script's own %s %s replaced by e9s's transaction for this %s.", strings.Join(p.replaced, " and "), verb, mode)
}

func startsTransaction(sql string) bool {
	words := SQLWords(sql)
	return len(words) > 0 && (words[0] == "begin" || (words[0] == "start" && len(words) > 1 && words[1] == "transaction"))
}

// endsTransaction reports a plain COMMIT, END, ROLLBACK or ABORT, which
// end the whole transaction; ROLLBACK TO a savepoint and AND CHAIN do not
// count.
func endsTransaction(sql string) bool {
	words := SQLWords(sql)
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "commit", "end", "rollback", "abort":
	default:
		return false
	}
	for index, word := range words[1:] {
		switch {
		case word == "to", word == "prepared":
			return false
		case word == "chain" && words[index] != "no":
			// words[index] is the word before this one.
			return false
		}
	}
	return true
}

func isSavepointControl(sql string) bool {
	words := SQLWords(sql)
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "savepoint", "release":
		return true
	case "rollback", "abort":
		return len(words) > 1 && words[1] == "to"
	}
	return false
}
