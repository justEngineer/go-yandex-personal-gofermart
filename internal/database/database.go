package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"embed"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	config "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/config"
	models "github.com/justEngineer/go-yandex-personal-gofermart/internal/models"
)

type Database struct {
	Connections *pgxpool.Pool
	mainContext *context.Context
}

//go:embed migrations/*.sql
var migrationSQL embed.FS

const migrationsDir = "migrations"

//go:generate mockgen -destination=../mocks/mock_database.go . Storage
type Storage interface {
	AddUser(ctx context.Context, registerData models.UserAuthData, hash string) (string, error)
	GetUser(ctx context.Context, login string) (models.UserInfo, error)
	GetOrders(ctx context.Context, userID string) ([]models.Order, error)
	AddOrder(ctx context.Context, externalOrderID string, userID string) (bool, error)
	GetUserBalance(ctx context.Context, userID string) (models.UserBalance, error)
	AddWithdrawal(ctx context.Context, userID string, withdrawal models.Withdrawal) error
	GetWithdrawals(ctx context.Context, userID string) ([]models.Withdrawal, error)
	GetOrdersInProgress(ctx context.Context) ([]models.Order, error)
	UpdateOrder(ctx context.Context, order models.AccrualInfo) error
	CloseConnections()
	GetOrderIDs() ([]string, error)
	ApplyAccural(accrual *models.AccrualInfo) error
}

// Получаем одно соединение для базы данных
func New(ctx context.Context, cfg *config.ServerConfig) (Storage, error) {
	connect, err := pgxpool.New(ctx, cfg.DBConnection)
	db := Database{Connections: connect, mainContext: &ctx}
	if err != nil {
		return &db, err
	} else {
		err = db.applyMigrations(cfg)
	}
	return &db, err
}

func (d *Database) applyMigrations(cfg *config.ServerConfig) error {
	driver, err := iofs.New(migrationSQL, migrationsDir)
	if err != nil {
		return fmt.Errorf("unable to apply db migrations: %v", err)
	}

	migrator, err := migrate.NewWithSourceInstance("iofs", driver, cfg.DBConnection)
	if err != nil {
		return fmt.Errorf("unable to create migration: %v", err)
	}
	defer migrator.Close()

	err = migrator.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Printf("No new migrations applied")
			return nil
		}
		return fmt.Errorf("unable to apply migrations %v", err)
	}
	return nil
}

func (d *Database) Ping() error {
	ctx, cancel := context.WithTimeout(*d.mainContext, 1*time.Second)
	defer cancel()
	err := d.Connections.Ping(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) CloseConnections() {
	defer d.Connections.Close()
}

func (d *Database) AddUser(ctx context.Context, user models.UserAuthData, hash string) (string, error) {
	row := d.Connections.QueryRow(ctx, "SELECT id FROM users WHERE login = $1", user.Login)
	var userID sql.NullString
	err := row.Scan(&userID)
	if err != nil && userID.Valid {
		log.Printf("Got error %s", err.Error())
		return "", err
	}
	if userID.Valid {
		log.Printf("user with login %s is existing", user.Login)
		return "", err
	}
	row = d.Connections.QueryRow(ctx, "INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id", user.Login, hash)
	if err := row.Scan(&userID); err != nil {
		log.Printf("error %s", err.Error())
		return "", err
	}
	log.Printf("userID %v", userID)
	if userID.Valid {
		userIDValue := userID.String
		log.Printf("new userID %s", userIDValue)
		return userIDValue, nil
	}
	return "", err
}

func (d *Database) GetUser(ctx context.Context, login string) (models.UserInfo, error) {
	row := d.Connections.QueryRow(ctx, "SELECT id, login, password_hash FROM users WHERE login = $1", login)
	var userData models.UserInfo
	err := row.Scan(&userData.ID, &userData.Login, &userData.Hash)
	if err != nil {
		log.Printf("could not get user data for login %s", login)
		return userData, err
	}
	return userData, nil
}

func (d *Database) AddOrder(ctx context.Context, id string, u string) (bool, error) {
	row := d.Connections.QueryRow(ctx, "SELECT user_id FROM orders WHERE external_id = $1", id)
	var orderUserID sql.NullString
	err := row.Scan(&orderUserID)
	if err != nil && orderUserID.Valid {
		log.Printf("error while querying %s", err.Error())
		return false, err
	}
	if orderUserID.Valid {
		if orderUserID.String == u {
			log.Printf("same userID %s for orderID %s", u, id)
			return true, fmt.Errorf("this order is exist the user, userID %s for orderID %s", u, id)
		} else {
			log.Printf("another userID %s (instead of %s) for orderID %s", orderUserID.String, u, id)
			return false, fmt.Errorf("this order is exist another user, userID %s for orderID %s", u, id)
		}
	}
	log.Printf("order with id %v not found in database", id)
	row = d.Connections.QueryRow(
		ctx,
		"INSERT INTO orders (user_id, status, external_id) VALUES ($1, $2, $3) RETURNING id",
		u, "NEW", id,
	)
	var orderID string
	err = row.Scan(&orderID)
	if err != nil {
		log.Printf("error while adding new order: %s", err.Error())
		return false, err
	}
	log.Printf("new order with id %s added", orderID)
	return true, nil
}

func (d *Database) GetOrders(ctx context.Context, u string) ([]models.Order, error) {
	rows, err := d.Connections.Query(ctx, "SELECT external_id, status, amount, registered_at FROM orders WHERE user_id = $1", u)
	if err != nil {
		log.Printf("error: %s", err.Error())
		return nil, err
	}
	defer rows.Close()
	orderList := make([]models.Order, 0)
	for rows.Next() {
		var orderFromDBVal models.OrderInfo
		err := rows.Scan(&orderFromDBVal.ID, &orderFromDBVal.Status, &orderFromDBVal.Accrual, &orderFromDBVal.UploadedAt)
		if err != nil {
			log.Printf("error: %s", err.Error())
			return nil, err
		}
		order := models.Order{ID: orderFromDBVal.ID, Status: orderFromDBVal.Status, UploadedAt: orderFromDBVal.UploadedAt}
		if orderFromDBVal.Accrual.Valid {
			order.Accrual = orderFromDBVal.Accrual.Float64
		}
		orderList = append(orderList, order)
	}
	if err := rows.Err(); err != nil {
		log.Printf("error: %s", err.Error())
		return nil, err
	}
	return orderList, nil
}

func (d *Database) GetUserBalance(ctx context.Context, u string) (models.UserBalance, error) {
	log.Printf("userID %s", u)
	sumOrdersRow := d.Connections.QueryRow(ctx, "SELECT sum(amount) FROM orders WHERE user_id = $1", u)
	sumWithdrawalsRow := d.Connections.QueryRow(ctx, "SELECT sum(amount) FROM withdrawal WHERE user_id = $1", u)
	var sumOrders sql.NullFloat64
	var sumWithdrawals sql.NullFloat64
	err := sumOrdersRow.Scan(&sumOrders)
	if err != nil && sumOrders.Valid {
		log.Printf("error get sumOrders: %s", err.Error())
		return models.UserBalance{Current: 0, Withdrawn: 0}, err
	}
	var resultBalance models.UserBalance
	if !sumOrders.Valid {
		log.Printf("resultBalance is empty")
		resultBalance.Current = 0
	} else {
		resultBalance.Current = sumOrders.Float64
	}
	err = sumWithdrawalsRow.Scan(&sumWithdrawals)

	if err != nil && sumWithdrawals.Valid {
		log.Printf("error get sumWithdrawals: %s", err.Error())
		return models.UserBalance{Current: 0, Withdrawn: 0}, err
	}
	if !sumWithdrawals.Valid {
		log.Printf("resultBalance is empty")
		resultBalance.Withdrawn = 0
	} else {
		resultBalance.Withdrawn = sumWithdrawals.Float64
	}
	resultBalance.Current -= resultBalance.Withdrawn
	log.Printf("balance %v", resultBalance)
	return resultBalance, nil
}

func (d *Database) AddWithdrawal(ctx context.Context, u string, w models.Withdrawal) error {
	userBalance, err := d.GetUserBalance(ctx, u)
	if err != nil {
		log.Printf("error while getting status %v", http.StatusInternalServerError)
		return err
	}
	if userBalance.Current < w.Sum {
		log.Printf("got less bonus points %v than expected %v", userBalance.Current, w.Sum)
		return err
	}
	var withdrawalID string
	row := d.Connections.QueryRow(ctx,
		"INSERT INTO withdrawal (user_id, amount, external_id) VALUES ($1, $2, $3) RETURNING id",
		u, w.Sum, w.Order,
	)
	err = row.Scan(&withdrawalID)
	if err != nil {
		log.Printf("error %s", err.Error())
		return err
	}
	log.Printf("new withdrawal %s", withdrawalID)
	return nil
}

func (d *Database) GetWithdrawals(ctx context.Context, u string) ([]models.Withdrawal, error) {
	rows, err := d.Connections.Query(ctx, "SELECT external_id, amount, registered_at FROM withdrawal WHERE user_id = $1", u)
	if err != nil {
		log.Printf("error %s", err.Error())
		return make([]models.Withdrawal, 0), err
	}
	defer rows.Close()
	withdrawalList := make([]models.Withdrawal, 0)
	for rows.Next() {
		var withdrawal models.Withdrawal
		err = rows.Scan(&withdrawal.Order, &withdrawal.Sum, &withdrawal.ProcessedAt)
		if err != nil {
			log.Printf("error %s", err.Error())
			return make([]models.Withdrawal, 0), err
		}
		withdrawalList = append(withdrawalList, withdrawal)
	}
	if err := rows.Err(); err != nil {
		log.Printf("error: %s", err.Error())
		return nil, err
	}
	return withdrawalList, nil
}

func (d *Database) GetOrdersInProgress(ctx context.Context) ([]models.Order, error) {
	rows, err := d.Connections.Query(ctx, "SELECT external_id, status, amount from orders where status not in ('INVALID', 'PROCESSED')")

	if err != nil {
		log.Printf("error %s", err.Error())
		return make([]models.Order, 0), err
	}
	defer rows.Close()
	orderList := make([]models.Order, 0)
	for rows.Next() {
		var orderFromDBVal models.OrderInfo
		err = rows.Scan(&orderFromDBVal.ID, &orderFromDBVal.Status, &orderFromDBVal.Accrual)
		if err != nil {
			log.Printf("error %s", err.Error())
			return make([]models.Order, 0), err
		}
		order := models.Order{ID: orderFromDBVal.ID, Status: orderFromDBVal.Status}
		if orderFromDBVal.Accrual.Valid {
			order.Accrual = orderFromDBVal.Accrual.Float64
		}
		orderList = append(orderList, order)
	}
	if err := rows.Err(); err != nil {
		log.Printf("error: %s", err.Error())
		return nil, err
	}
	log.Printf("orders %v", orderList)
	return orderList, nil
}

func (d *Database) UpdateOrder(ctx context.Context, o models.AccrualInfo) error {
	tx, err := d.Connections.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Printf("error %s", err.Error())
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE orders SET status = $1, amount = $2 where external_id = $3", o.Status, o.Accrual, o.Order); err != nil {
		if err = tx.Rollback(ctx); err != nil {
			log.Fatalf("insert to url, need rollback, %v", err)
			return err
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("unable to commit: %v", err)
		return err
	}
	return nil
}

func (d *Database) GetOrderIDs() ([]string, error) {
	orders, err := d.GetOrdersInProgress(context.Background())
	if err != nil {
		return nil, err
	}
	orderIDs := make([]string, 0)
	for _, order := range orders {
		num, err := strconv.ParseInt(order.ID, 10, 64)
		if err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, strconv.FormatInt(num, 10))
	}
	return orderIDs, nil
}

func (d *Database) ApplyAccural(accrual *models.AccrualInfo) error {
	order := models.AccrualInfo{Order: accrual.Order, Accrual: accrual.Accrual, Status: accrual.Status}
	err := d.UpdateOrder(context.Background(), order)
	if err != nil {
		return err
	}
	return nil
}
