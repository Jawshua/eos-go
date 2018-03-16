package eosapi

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// For reference:
// https://github.com/mithrilcoin-io/EosCommander/blob/master/app/src/main/java/io/mithrilcoin/eoscommander/data/remote/model/types/EosByteWriter.java

type Name string
type AccountName Name
type PermissionName Name
type ActionName Name
type TableName Name

func (acct AccountName) MarshalBinary() ([]byte, error)    { return Name(acct).MarshalBinary() }
func (acct PermissionName) MarshalBinary() ([]byte, error) { return Name(acct).MarshalBinary() }
func (acct ActionName) MarshalBinary() ([]byte, error)     { return Name(acct).MarshalBinary() }
func (acct TableName) MarshalBinary() ([]byte, error)      { return Name(acct).MarshalBinary() }
func (acct Name) MarshalBinary() ([]byte, error) {
	val, err := StringToName(string(acct))
	if err != nil {
		return nil, err
	}
	var out [8]byte
	binary.LittleEndian.PutUint64(out[:8], val)
	return out[:], nil
}

func (n *AccountName) UnmarshalBinary(data []byte) error {
	*n = AccountName(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}
func (n *Name) UnmarshalBinary(data []byte) error {
	*n = Name(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}
func (n *PermissionName) UnmarshalBinary(data []byte) error {
	*n = PermissionName(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}
func (n *ActionName) UnmarshalBinary(data []byte) error {
	*n = ActionName(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}
func (n *TableName) UnmarshalBinary(data []byte) error {
	*n = TableName(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}

func (AccountName) UnmarshalBinarySize() int    { return 8 }
func (PermissionName) UnmarshalBinarySize() int { return 8 }
func (ActionName) UnmarshalBinarySize() int     { return 8 }
func (TableName) UnmarshalBinarySize() int      { return 8 }
func (Name) UnmarshalBinarySize() int           { return 8 }

// CurrencyName

type CurrencyName string

func (c CurrencyName) MarshalBinary() ([]byte, error) {
	out := make([]byte, 7, 7)
	fmt.Println("AMAM", out)
	copy(out, []byte(c))
	return out, nil
}

func (c *CurrencyName) UnmarshalBinary(data []byte) error {
	*c = CurrencyName(strings.TrimRight(string(data), "\x00"))
	return nil
}
func (CurrencyName) UnmarshalBinarySize() int { return 7 }

// Asset

type Asset struct {
	Precision int    `struc:"uint8"`
	Symbol    string `struc:"[7]byte"`
} // decode "1000.0000 EOS" as `Asset{Amount: 10000000, Symbol: "EOS", Precision: 4}`

func (a *Asset) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	return nil
}

// NOT RIGHT SIGNATURE:
func (a *Asset) MarshalJSON() (data []byte, err error) {
	return nil, nil
}

type AccountResp struct {
	AccountName AccountName  `json:"account"`
	Permissions []Permission `json:"permissions"`
}

type CurrencyBalanceResp struct {
	EOSBalance        Asset    `json:"eos_balance"`
	StakedBalance     Asset    `json:"staked_balance"`
	UnstakingBalance  Asset    `json:"unstaking_balance"`
	LastUnstakingTime JSONTime `json:"last_unstaking_time"`
}

type PublicKey string

type Permission struct {
	PermName     string    `json:"perm_name"`
	Parent       string    `json:"parent"`
	RequiredAuth Authority `json:"required_auth"`
}

type PermissionLevel struct {
	Actor      AccountName    `json:"actor"`
	Permission PermissionName `json:"permission"`
}

type PermissionLevelWeight struct {
	Permission PermissionLevel `json:"permission"`
	Weight     uint16          `json:"weight"`
}

type Authority struct {
	Threshold uint32                  `json:"threshold"`
	Accounts  []PermissionLevelWeight `json:"accounts"`
	Keys      []KeyWeight             `json:"keys"`
}

type KeyWeight struct {
	PublicKey PublicKey `json:"public_key"`
	Weight    uint16    `json:"weight"`
}

type Code struct {
	AccountName AccountName `json:"account_name"`
	CodeHash    string      `json:"code_hash"`
	WAST        string      `json:"wast"` // TODO: decode into Go ast, see https://github.com/go-interpreter/wagon
	ABI         ABI         `json:"abi"`
}

type Action struct {
	Account       AccountName       `json:"account"`
	Name          ActionName        `json:"name"`
	Authorization []PermissionLevel `json:"authorization,omitempty"`
	Data          HexBytes          `json:"data,omitempty"` // as HEX when we receive it.. FIXME: decode from hex directly.. and encode back plz!
	Fields        interface{}       `json:"-"`
}

type action struct {
	Account       AccountName       `json:"account"`
	Name          ActionName        `json:"name"`
	Authorization []PermissionLevel `json:"authorization,omitempty"`
	Data          HexBytes          `json:"data,omitempty"`
}

// with an action type registry somewhere ?

var registeredActions = map[AccountName]map[ActionName]reflect.Type{}

func init() {
	RegisterAction(AccountName("eosio"), ActionName("transfer"), &Transfer{})
	RegisterAction(AccountName("eosio"), ActionName("issue"), &Issue{})
}

// Registers Action objects..
func RegisterAction(accountName AccountName, actionName ActionName, obj interface{}) {
	// TODO: lock or som'th.. unless we never call after boot time..
	if registeredActions[accountName] == nil {
		registeredActions[accountName] = make(map[ActionName]reflect.Type)
	}
	registeredActions[accountName][actionName] = reflect.ValueOf(obj).Type()
}

func (a *Action) UnmarshalJSON(v []byte) (err error) {
	// load Account, Name, Authorization, Data
	// and then unpack other fields in a struct based on `Name` and `AccountName`..
	var newAct *action
	if err = json.Unmarshal(v, &newAct); err != nil {
		return
	}

	a.Account = newAct.Account
	a.Name = newAct.Name
	a.Authorization = newAct.Authorization
	a.Data = newAct.Data

	actionMap := registeredActions[a.Account]
	if actionMap == nil {
		return nil
	}

	objMap := actionMap[a.Name]
	if objMap == nil {
		return nil
	}

	obj := reflect.New(reflect.TypeOf(objMap))
	err = json.Unmarshal(v, &obj)
	if err != nil {
		return err
	}

	a.Fields = obj.Elem().Interface()

	return nil
}

func (a *Action) MarshalJSON() ([]byte, error) {
	// Start with the base-line Action fields.

	cnt, err := json.Marshal(&action{
		Account:       a.Account,
		Name:          a.Name,
		Authorization: a.Authorization,
		Data:          a.Data,
	})
	if err != nil {
		return nil, err
	}

	var keys1 map[string]interface{}
	err = json.Unmarshal(cnt, &keys1)
	if err != nil {
		return nil, err
	}

	// Merge in the `a.Fields` fields.

	cnt, err = json.Marshal(a.Fields)
	if err != nil {
		return nil, err
	}

	var keys2 map[string]interface{}
	err = json.Unmarshal(cnt, &keys2)
	if err != nil {
		return nil, err
	}

	for k, v := range keys2 {
		keys1[k] = v
	}

	return json.Marshal(keys1)
}

// JSONTime

type JSONTime struct {
	time.Time
}

const JSONTimeFormat = "2006-01-02T15:04:05"

func (t JSONTime) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", t.Format(JSONTimeFormat))), nil
}

func (t *JSONTime) UnmarshalJSON(data []byte) (err error) {
	if string(data) == "null" {
		return nil
	}

	t.Time, err = time.Parse(`"`+JSONTimeFormat+`"`, string(data))
	return err
}

func (t JSONTime) UnmarshalBinary(data []byte) error {
	t.Time = time.Unix(int64(binary.LittleEndian.Uint32(data)), 0)
	return nil
}

func (t JSONTime) MarshalBinary() ([]byte, error) {
	out := []byte{0, 0, 0, 0}
	binary.LittleEndian.PutUint32(out, uint32(t.Unix()))
	return out, nil
}

func (t JSONTime) MarshalBinarySize() int { return 4 }

// HexBytes

type HexBytes []byte

func (t HexBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(t))
}

func (t *HexBytes) UnmarshalJSON(data []byte) (err error) {
	var s string
	err = json.Unmarshal(data, &s)
	if err != nil {
		return
	}

	*t, err = hex.DecodeString(s)
	return
}

type ProducerChange struct {
}

type Cycle struct {
}

type GetTableRowsRequest struct {
	JSON       bool   `json:"json"`
	Scope      string `json:"scope"`
	Code       string `json:"code"`
	Table      string `json:"table"`
	TableKey   string `json:"table_key"`
	LowerBound string `json:"lower_bound"`
	UpperBound string `json:"upper_bount"`
	Limit      uint32 `json:"limit,omitempty"` // defaults to 10 => chain_plugin.hpp:struct get_table_rows_params
}

type Transaction struct { // WARN: is a `variant` in C++, can be a SignedTransaction or a Transaction.
	Expiration     JSONTime `json:"expiration,omitempty"`
	Region         uint16   `json:"region,omitempty"`
	RefBlockNum    uint16   `json:"ref_block_num,omitempty"`
	RefBlockPrefix uint32   `json:"ref_block_prefix,omitempty"`
	// number of 8 byte words this transaction can compress into
	PackedBandwidthWords    uint16    `json:"packed_bandwidth_words,omitempty"`
	ContextFreeCPUBandwidth uint16    `json:"context_free_cpu_bandwidth,omitempty"`
	ContextFreeActions      []*Action `json:"context_free_actions,omitempty"`
	Actions                 []*Action `json:"actions,omitempty"`
}

func (tx *Transaction) Fill(api *EOSAPI) error {
	if tx.RefBlockNum != 0 {
		return nil
	}

	info, err := api.GetInfo()
	if err != nil {
		return err
	}

	blockID, err := hex.DecodeString(info.HeadBlockID)
	if err != nil {
		return fmt.Errorf("decode hex: %s", err)
	}

	tx.RefBlockNum = uint16(binary.LittleEndian.Uint64(blockID[:8]))
	tx.RefBlockPrefix = uint32(binary.LittleEndian.Uint64(blockID[8:16]))
	/// TODO: configure somewhere the default time for transactions,
	/// etc.. add a `.Timeout` with that duration, default to 30
	/// seconds ?
	tx.Expiration = JSONTime{info.HeadBlockTime.Add(30 * time.Second)}
	return nil
}

type SignedTransaction struct {
	*Transaction

	Signatures      []string `json:"signatures"`
	ContextFreeData HexBytes `json:"context_free_data,omitempty"`
}

type DeferredTransaction struct {
	*Transaction

	SenderID   uint32      `json:"sender_id"`
	Sender     AccountName `json:"sender"`
	DelayUntil JSONTime    `json:"delay_until"`
}
