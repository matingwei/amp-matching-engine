package types

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gopkg.in/mgo.v2/bson"
)

// Wallet holds both the address and the private key of an ethereum account
type Wallet struct {
	ID         bson.ObjectId
	Address    common.Address
	PrivateKey *ecdsa.PrivateKey
	Admin      bool
}

// NewWallet returns a new wallet object corresponding to a random private key
func NewWallet() *Wallet {
	privateKey, _ := crypto.GenerateKey()
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	return &Wallet{
		Address:    address,
		PrivateKey: privateKey,
	}
}

// NewWalletFromPrivateKey returns a new wallet object corresponding
// to a given private key
func NewWalletFromPrivateKey(key string) *Wallet {
	privateKey, err := crypto.HexToECDSA(key)
	if err != nil {
		fmt.Printf("Error: %v", err)
	}

	return &Wallet{
		Address:    crypto.PubkeyToAddress(privateKey.PublicKey),
		PrivateKey: privateKey,
	}
}

// GetAddress returns the wallet address
func (w *Wallet) GetAddress() string {
	return w.Address.Hex()
}

// GetPrivateKey returns the wallet private key
func (w *Wallet) GetPrivateKey() string {
	return hex.EncodeToString(w.PrivateKey.D.Bytes())
}

func (w *Wallet) Validate() error {
	return nil
}

type WalletRecord struct {
	ID         bson.ObjectId `json:"id,omitempty" bson:"_id"`
	Address    string        `json:"address" bson:"address"`
	PrivateKey string        `json:"privateKey" bson:"privateKey"`
	Admin      bool          `json:"admin" bson:"admin"`
}

func (w *Wallet) GetBSON() (interface{}, error) {
	return WalletRecord{
		ID:         w.ID,
		Address:    w.Address.Hex(),
		PrivateKey: hex.EncodeToString(w.PrivateKey.D.Bytes()),
		Admin:      w.Admin,
	}, nil
}

func (w *Wallet) SetBSON(raw bson.Raw) error {
	decoded := &WalletRecord{}
	err := raw.Unmarshal(decoded)
	if err != nil {
		return err
	}

	w.ID = decoded.ID
	w.Address = common.HexToAddress(decoded.Address)
	w.PrivateKey, _ = crypto.HexToECDSA(decoded.PrivateKey)
	w.Admin = decoded.Admin
	return nil
}

// SignHash signs a hashed message with a wallet private key
// and returns it as a Signature object
func (w *Wallet) SignHash(h common.Hash) (*Signature, error) {
	message := crypto.Keccak256(
		[]byte("\x19Ethereum Signed Message:\n32"),
		h.Bytes(),
	)

	sigBytes, err := crypto.Sign(message, w.PrivateKey)
	if err != nil {
		return &Signature{}, err
	}

	sig := &Signature{
		R: common.BytesToHash(sigBytes[0:32]),
		S: common.BytesToHash(sigBytes[32:64]),
		V: sigBytes[64] + 27,
	}

	return sig, nil
}

// SignTrade signs and sets the signature of a trade with a wallet private key
func (w *Wallet) SignTrade(t *Trade) error {
	hash := t.ComputeHash()
	t.Hash = hash

	// sig, err := w.SignHash(hash)
	// if err != nil {
	// 	return err
	// }

	// t.Signature = sig
	return nil
}

func (w *Wallet) Print() {
	b, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		fmt.Println("Error: ", err)
	}

	fmt.Print(string(b))
}

// NewOrder (DEPRECATED - use the order factory instead) creates a new
// order from a wallet, compute the order hash and signs it with the
// wallet private key
// func (w *Wallet) NewOrder(id, amountBuy, amountSell int64, p TokenPair, ot OrderType) (*Order, error) {
// 	o := &Order{}
// 	tokenBuy := Token{}
// 	tokenSell := Token{}

// 	if ot == BUY {
// 		tokenBuy = p.QuoteToken
// 		tokenSell = p.BaseToken
// 	} else {
// 		tokenBuy = p.BaseToken
// 		tokenSell = p.QuoteToken
// 	}

// 	o.Id = id
// 	o.ExchangeAddress = config.Exchange
// 	o.TokenBuy = tokenBuy.Address
// 	o.TokenSell = tokenSell.Address
// 	o.SymbolBuy = tokenBuy.Symbol
// 	o.SymbolSell = tokenSell.Symbol
// 	o.AmountBuy = big.NewInt(int64(amountBuy))
// 	o.AmountSell = big.NewInt(int64(amountSell))
// 	o.Expires = big.NewInt(0)
// 	o.FeeMake = big.NewInt(0)
// 	o.FeeTake = big.NewInt(0)
// 	o.Nonce = big.NewInt(0)
// 	o.Maker = w.Address
// 	o.PairID = p.ID
// 	o.Price = 0
// 	o.Amount = 0

// 	hash := o.ComputeHash()
// 	o.Hash = hash

// 	sig, err := w.SignHash(hash)
// 	if err != nil {
// 		return nil, err
// 	}w
// 	o.Signature = sig

// 	return o, nil
// }

// NewTrade (DEPRECATED - use the order factory instead) creates a new
// trade from a wallet and a given order, compute the trade hash and
// signs it with the wallet private key
// func (w *Wallet) NewTrade(o *Order, amount int64) (*Trade, error) {
// 	trade := &Trade{}

// 	trade.OrderHash = o.Hash
// 	trade.Amount = big.NewInt(int64(amount))
// 	trade.TradeNonce = big.NewInt(0)
// 	trade.Taker = w.Address

// 	hash := trade.ComputeHash()
// 	trade.Hash = hash

// 	sig, err := w.SignHash(hash)
// 	if err != nil {
// 		return nil, err
// 	}

// 	trade.Signature = sig
// 	return trade, nil
// }
