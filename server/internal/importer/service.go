package importer

import (
	"context"
	"runtime"
	"sync"

	"dtr/server/internal/auth"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type Created struct {
	EmployeeNo string `json:"employee_no"`
	FullName   string `json:"full_name"`
	TempPIN    string `json:"temp_pin"`
}

// MarkExisting flags rows whose employee number is already taken.
func (s *Service) MarkExisting(ctx context.Context, rows []Row) error {
	var nos []string
	for _, r := range rows {
		if r.Status == "ok" {
			nos = append(nos, r.EmployeeNo)
		}
	}
	if len(nos) == 0 {
		return nil
	}
	found, err := s.pool.Query(ctx, `SELECT employee_no FROM employees WHERE employee_no = ANY($1)`, nos)
	if err != nil {
		return err
	}
	defer found.Close()
	taken := map[string]bool{}
	for found.Next() {
		var no string
		if err := found.Scan(&no); err != nil {
			return err
		}
		taken[no] = true
	}
	for i := range rows {
		if rows[i].Status == "ok" && taken[rows[i].EmployeeNo] {
			rows[i].Status, rows[i].Message = "duplicate", "this employee number already exists"
		}
	}
	return found.Err()
}

// Commit creates an account for every row that is still ok, with a temporary
// PIN each that the employee must change at first sign-in. All or nothing.
func (s *Service) Commit(ctx context.Context, rows []Row) ([]Created, error) {
	var todo []int
	for i, r := range rows {
		if r.Status == "ok" {
			todo = append(todo, i)
		}
	}
	if len(todo) == 0 {
		return nil, nil
	}

	// argon2 is deliberately slow and memory hungry: hash in parallel, but
	// not so wide that a big import exhausts memory.
	pins := make([]string, len(todo))
	hashes := make([]string, len(todo))
	errs := make([]error, len(todo))
	var wg sync.WaitGroup
	sem := make(chan struct{}, min(4, runtime.NumCPU()))
	for n := range todo {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			pin, err := auth.TempPIN()
			if err == nil {
				pins[n] = pin
				hashes[n], err = auth.HashPIN(pin)
			}
			errs[n] = err
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	batch := &pgx.Batch{}
	for n, i := range todo {
		r := rows[i]
		batch.Queue(`INSERT INTO employees (employee_no, full_name, position, office, role, employment, pin_hash, must_change_pin)
			VALUES ($1,$2,$3,$4,$5::employee_role,$6::employment_status,$7,TRUE)`,
			r.EmployeeNo, r.FullName, r.Position, r.Office, r.Role, r.Employment, hashes[n])
	}
	br := tx.SendBatch(ctx, batch)
	for range todo {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return nil, err
		}
	}
	if err := br.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	created := make([]Created, len(todo))
	for n, i := range todo {
		created[n] = Created{EmployeeNo: rows[i].EmployeeNo, FullName: rows[i].FullName, TempPIN: pins[n]}
	}
	return created, nil
}
