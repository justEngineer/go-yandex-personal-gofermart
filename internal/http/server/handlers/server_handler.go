package server

import (
	"bytes"
	//"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"encoding/json"
	"io"

	"github.com/go-chi/chi/v5"
	chi_middleware "github.com/go-chi/chi/v5/middleware"

	database "github.com/justEngineer/go-yandex-personal-gofermart/internal/database"
	config "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/config"
	middleware "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/middleware"
	logger "github.com/justEngineer/go-yandex-personal-gofermart/internal/logger"
	models "github.com/justEngineer/go-yandex-personal-gofermart/internal/models"
	security "github.com/justEngineer/go-yandex-personal-gofermart/internal/security"
)

type DataTypeParametr interface {
	interface{} | []interface{}
}

func ParseJSON[DataType DataTypeParametr](w http.ResponseWriter, r *http.Request) (bool, DataType) {
	var parsedData DataType
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		http.Error(w, fmt.Sprintf("Error while reading request body: %s", err.Error()), http.StatusBadRequest)
		return false, parsedData
	}
	if err := json.Unmarshal(buf.Bytes(), &parsedData); err != nil {
		http.Error(w, fmt.Sprintf("Error while unmarshaling data %s", err.Error()), http.StatusBadRequest)
		return false, parsedData
	}
	return true, parsedData
}

func EncodeToJSONAndWriteResponse(w http.ResponseWriter, data any) {
	marshalled, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error while JSON marshalling: %s", err.Error())
		http.Error(w, "Error while JSON marshalling", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(marshalled)
}

type Handler struct {
	config    *config.ServerConfig
	appLogger *logger.Logger
	storage   database.Storage
}

func New(config *config.ServerConfig, log *logger.Logger, conn database.Storage) *Handler {
	return &Handler{config, log, conn}
}

func ValidateOrderID(order string) bool {
	var sum int
	var invert bool

	for i := len(order) - 1; i >= 0; i-- {
		digit, err := strconv.Atoi(string(order[i]))
		if err != nil {
			return false
		}
		if invert {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		invert = !invert
	}
	return sum%10 == 0
}

func (h *Handler) GetRouter(auth *middleware.AuthMiddleware) http.Handler {
	r := chi.NewRouter()
	r.Use(h.appLogger.RequestLogger)
	r.Use(chi_middleware.Recoverer)

	r.Route("/api/user", func(r chi.Router) {
		//      /api/user/register — регистрация пользователя;
		r.With(middleware.JSONContentHandler).Post("/register", h.NewUserRegistration)
		//      /api/user/login — аутентификация пользователя;
		r.With(middleware.JSONContentHandler).Post("/login", h.UserAuthentication)
		//      /api/user/balance/withdraw — запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа;
		r.With(middleware.JSONContentHandler, auth.Handler).Post("/balance/withdraw", h.ExecutePayment)
		//      /api/user/orders — загрузка пользователем номера заказа для расчёта;
		r.With(middleware.TextContentHandler, auth.Handler).Post("/orders", h.CreateNewOrder)
		//      /api/user/orders — получение списка загруженных пользователем номеров заказов, статусов их обработки и информации о начислениях;
		r.With(auth.Handler).Get("/orders", h.GetOrders)
		//      /api/user/balance — получение текущего баланса счёта баллов лояльности пользователя;
		r.With(auth.Handler).Get("/balance", h.GetBalance)
		//      /api/user/withdrawals — получение информации о выводе средств с накопительного счёта пользователем.
		r.With(auth.Handler).Get("/withdrawals", h.GetPaymentHistory)
	})
	return r
}

func (h *Handler) NewUserRegistration(w http.ResponseWriter, r *http.Request) {
	res, data := ParseJSON[models.UserAuthData](w, r)
	if !res {
		return
	}
	if data.Login == "" || data.Password == "" {
		http.Error(w, "Request doesn't contain login or password", http.StatusBadRequest)
		return
	}

	token, err := security.GenerateToken(data.Login, h.config.SHA256Key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error while generating jwt token: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	passwordHash, err := security.GetHashedPassword(&data.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = h.storage.AddUser(r.Context(), data, passwordHash)
	if err != nil {
		log.Println("Error register user")
		http.Error(w, "Error register user", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))
	w.WriteHeader(http.StatusOK)
	w.Write(make([]byte, 0))
}

func (h *Handler) UserAuthentication(w http.ResponseWriter, r *http.Request) {
	res, authData := ParseJSON[models.UserAuthData](w, r)
	if !res {
		return
	}
	userData, err := h.storage.GetUser(r.Context(), authData.Login)
	if err != nil {
		log.Println("Error get user by login")
		http.Error(w, "Error get user by login", http.StatusUnauthorized)
		return
	}
	if err = security.VerifyPassword(&authData, &userData); err == nil {
		token, err := security.GenerateToken(authData.Login, h.config.SHA256Key)
		if err != nil {
			log.Println("Generate token error")
			http.Error(w, "Generate token login", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))
		w.WriteHeader(http.StatusOK)
		w.Write(make([]byte, 0))
	} else {
		log.Println("Wrong login/password combination")
		http.Error(w, fmt.Sprintf("Wrong login/password combination: %s", err.Error()), http.StatusUnauthorized)
	}
}

func (h *Handler) CreateNewOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error while reading body: ")
	}

	if !ValidateOrderID(string(orderID)) {
		log.Printf("Wrong order ID %s", orderID)
		http.Error(w, "Wrong order ID", http.StatusUnprocessableEntity)
		return
	}
	userID := r.Context().Value(models.UserInfoKey).(string)
	success, err := h.storage.AddOrder(r.Context(), string(orderID), userID)
	if err != nil {
		log.Printf("Error add order into db, %d", http.StatusInternalServerError)
	}
	if success && err != nil {
		w.WriteHeader(http.StatusOK)
		return
	} else if !success {
		w.WriteHeader(http.StatusConflict)
		http.Error(w, "Error add order into db", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write(make([]byte, 0))
}

func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request) {
	log.Println("Got GetOrders request")
	userID := r.Context().Value(models.UserInfoKey).(string)
	orders, err := h.storage.GetOrders(r.Context(), userID)
	if err != nil {
		log.Printf("Error %v", err)
		http.Error(w, "bad status code", http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		log.Println("orders is empty")
		http.Error(w, "no orders for this user", http.StatusNoContent)
		return
	}
	log.Printf("orders %v", orders)
	EncodeToJSONAndWriteResponse(w, orders)
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userBalance, err := h.storage.GetUserBalance(r.Context(), r.Context().Value(models.UserInfoKey).(string))
	if err != nil {
		http.Error(w, "Error get user balance", http.StatusInternalServerError)
		return
	}
	EncodeToJSONAndWriteResponse(w, userBalance)
}

func (h *Handler) ExecutePayment(w http.ResponseWriter, r *http.Request) {
	res, withdrawal := ParseJSON[models.Withdrawal](w, r)
	if !res {
		return
	}
	if !ValidateOrderID(withdrawal.Order) {
		log.Printf("Wrong order ID %s", withdrawal.Order)
		http.Error(w, "Wrong order ID", http.StatusUnprocessableEntity)
		return
	}
	userID := r.Context().Value(models.UserInfoKey).(string)
	err := h.storage.AddWithdrawal(r.Context(), userID, withdrawal)
	if err != nil {
		log.Printf("errorCode %v", http.StatusInternalServerError)
		http.Error(w, "Error from AddWithdrawalForUser", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(make([]byte, 0))
}

func (h *Handler) GetPaymentHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(models.UserInfoKey).(string)
	withdrawals, err := h.storage.GetWithdrawals(r.Context(), userID)
	if err != nil {
		log.Printf("errCode %v", http.StatusInternalServerError)
		http.Error(w, "err", http.StatusInternalServerError)
		return
	}
	if len(withdrawals) == 0 {
		http.Error(w, "Withdrawals for this user is not found", http.StatusNoContent)
		return
	}
	EncodeToJSONAndWriteResponse(w, withdrawals)
}
