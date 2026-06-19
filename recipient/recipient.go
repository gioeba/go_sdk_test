// Package recipient exposes the Hinkal SDK private-recipient and stealth-address helpers.
package recipient

import pretx "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"

var (
	IsValidPrivateAddress                  = pretx.IsValidPrivateAddress
	ConstructStealthAddressStructure       = pretx.ConstructStealthAddressStructure
	GetRecipientInfoFromUserKeys           = pretx.GetRecipientInfoFromUserKeys
	GetStealthAddressStructureFromUserKeys = pretx.GetStealthAddressStructureFromUserKeys
)
