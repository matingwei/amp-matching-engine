package types

import (
	"fmt"
	"math/big"
	"time"

	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-ozzo/ozzo-validation"
	"gopkg.in/mgo.v2/bson"
)

// Account corresponds to a single Ethereum address. It contains a list of token balances for that address
type Account struct {
	ID            bson.ObjectId                    `json:"-" bson:"_id"`
	Address       common.Address                   `json:"address" bson:"address"`
	TokenBalances map[common.Address]*TokenBalance `json:"tokenBalances" bson:"tokenBalances"`
	IsBlocked     bool                             `json:"isBlocked" bson:"isBlocked"`
	CreatedAt     time.Time                        `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time                        `json:"updatedAt" bson:"updatedAt"`
}

// TokenBalance holds the Balance, Allowance and the Locked balance values for a single Ethereum token
// Balance, Allowance and Locked Balance are stored as big.Int as they represent uint256 values
type TokenBalance struct {
	ID            bson.ObjectId  `json:"id" bson:"id"`
	Address       common.Address `json:"address" bson:"address"`
	Symbol        string         `json:"symbol" bson:"symbol"`
	Balance       *big.Int       `json:"balance" bson:"balance"`
	Allowance     *big.Int       `json:"allowance" bson:"allowance"`
	LockedBalance *big.Int       `json:"lockedBalance" bson:"lockedBalance"`
}

// AccountRecord corresponds to what is stored in the DB. big.Ints are encoded as strings
type AccountRecord struct {
	ID            bson.ObjectId                 `json:"id" bson:"_id"`
	Address       string                        `json:"address" bson:"address"`
	TokenBalances map[string]TokenBalanceRecord `json:"tokenBalances" bson:"tokenBalances"`
	IsBlocked     bool                          `json:"isBlocked" bson:"isBlocked"`
	CreatedAt     time.Time                     `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time                     `json:"updatedAt" bson:"updatedAt"`
}

// TokenBalanceRecord corresponds to a TokenBalance struct that is stored in the DB. big.Ints are encoded as strings
type TokenBalanceRecord struct {
	ID            bson.ObjectId `json:"id" bson:"id"`
	Address       string        `json:"address" bson:"address"`
	Symbol        string        `json:"symbol" bson:"symbol"`
	Balance       string        `json:"balance" bson:"balance"`
	Allowance     string        `json:"allowance" bson:"allowance"`
	LockedBalance string        `json:"lockedBalance" bson:"lockedBalance"`
}

// GetBSON implements bson.Getter
func (a *Account) GetBSON() (interface{}, error) {
	tokenBalances := make(map[string]TokenBalanceRecord)

	for key, value := range a.TokenBalances {
		tokenBalances[key.Hex()] = TokenBalanceRecord{
			ID:            value.ID,
			Address:       value.Address.Hex(),
			Symbol:        value.Symbol,
			Balance:       value.Balance.String(),
			Allowance:     value.Allowance.String(),
			LockedBalance: value.LockedBalance.String(),
		}
	}

	return AccountRecord{
		ID:            a.ID,
		Address:       a.Address.Hex(),
		TokenBalances: tokenBalances,
	}, nil
}

// SetBSON implemenets bson.Setter
func (a *Account) SetBSON(raw bson.Raw) error {
	decoded := &AccountRecord{}

	err := raw.Unmarshal(decoded)
	if err != nil {
		return err
	}

	a.TokenBalances = make(map[common.Address]*TokenBalance)
	for key, value := range decoded.TokenBalances {

		balance := new(big.Int)
		balance, _ = balance.SetString(value.Balance, 10)
		allowance := new(big.Int)
		allowance, _ = allowance.SetString(value.Allowance, 10)
		lockedBalance := new(big.Int)
		lockedBalance, _ = lockedBalance.SetString(value.LockedBalance, 10)

		a.TokenBalances[common.HexToAddress(key)] = &TokenBalance{
			ID:            value.ID,
			Address:       common.HexToAddress(value.Address),
			Symbol:        value.Symbol,
			Balance:       balance,
			Allowance:     allowance,
			LockedBalance: lockedBalance,
		}
	}

	a.ID = decoded.ID
	a.Address = common.HexToAddress(decoded.Address)
	a.IsBlocked = decoded.IsBlocked
	a.CreatedAt = decoded.CreatedAt
	a.UpdatedAt = decoded.UpdatedAt

	return nil
}

// JSON Marshal/Unmarshal interface

// MarshalJSON implements the json.Marshal interface
func (a *Account) MarshalJSON() ([]byte, error) {
	account := map[string]interface{}{
		"id":        a.ID,
		"address":   a.Address,
		"isBlocked": a.IsBlocked,
		"createdAt": a.CreatedAt.String(),
		"updatedAt": a.UpdatedAt.String(),
	}
	tokenBalance := make(map[string]interface{})
	for address, balance := range a.TokenBalances {
		tokenBalance[address.Hex()] = map[string]interface{}{
			"id":            balance.ID.Hex(),
			"address":       balance.Address.Hex(),
			"symbol":        balance.Symbol,
			"balance":       balance.Balance.String(),
			"allowance":     balance.Allowance.String(),
			"lockedBalance": balance.LockedBalance.String(),
		}
	}
	account["tokenBalances"] = tokenBalance
	return json.Marshal(account)
}

func (a *Account) UnmarshalJSON(b []byte) error {
	account := map[string]interface{}{}
	err := json.Unmarshal(b, &account)
	if err != nil {
		return err
	}
	if account["id"] != nil {
		a.ID = bson.ObjectIdHex(account["id"].(string))
	}
	if account["address"] != nil {
		a.Address = common.HexToAddress(account["address"].(string))
	}
	if account["tokenBalances"] != nil {
		tokenBalances := account["tokenBalances"].(map[string]interface{})
		a.TokenBalances = make(map[common.Address]*TokenBalance)
		for address, balance := range tokenBalances {
			if !common.IsHexAddress(address) {
				continue
			}
			tokenBalance := balance.(map[string]interface{})
			tb := &TokenBalance{}
			if tokenBalance["id"] != nil && bson.IsObjectIdHex(tokenBalance["id"].(string)) {
				tb.ID = bson.ObjectIdHex(tokenBalance["id"].(string))
			}
			if tokenBalance["address"] != nil && common.IsHexAddress(tokenBalance["address"].(string)) {
				tb.Address = common.HexToAddress(tokenBalance["address"].(string))
			}
			if tokenBalance["symbol"] != nil {
				tb.Symbol = tokenBalance["symbol"].(string)
			}
			tb.Balance = new(big.Int)
			tb.Allowance = new(big.Int)
			tb.LockedBalance = new(big.Int)

			if tokenBalance["balance"] != nil {
				tb.Balance.UnmarshalJSON([]byte(tokenBalance["balance"].(string)))
			}
			if tokenBalance["allowance"] != nil {
				tb.Allowance.UnmarshalJSON([]byte(tokenBalance["allowance"].(string)))
			}
			if tokenBalance["lockedBalance"] != nil {
				tb.LockedBalance.UnmarshalJSON([]byte(tokenBalance["lockedBalance"].(string)))
			}
			a.TokenBalances[common.HexToAddress(address)] = tb
		}
	}
	return nil
}

// Validate enforces the account model
func (a Account) Validate() error {
	return validation.ValidateStruct(&a,
		validation.Field(&a.Address, validation.Required),
	)
}

func (a *Account) Print() {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		fmt.Println("Error: ", err)
	}

	fmt.Print(string(b))
}

func (a *AccountRecord) Print() {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		fmt.Println("Error: ", err)
	}

	fmt.Print(string(b))
}

func (t *TokenBalance) Print() {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		fmt.Println("Error: ", err)
	}

	fmt.Print(string(b))
}

func (t *TokenBalanceRecord) Print() {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		fmt.Println("Error: ", err)
	}

	fmt.Print(string(b))
}
