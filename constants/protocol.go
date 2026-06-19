package constants

const (
	ZeroAddress         = "0x0000000000000000000000000000000000000000"
	SolanaNativeAddress = "11111111111111111111111111111111"

	// OneInchZeroAddress is the 1inch sentinel address for the native gas token.
	OneInchZeroAddress = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

	// HinkalSwapVariableRate is the variable fee rate (basis points) the relayer charges on swaps.
	HinkalSwapVariableRate int64 = 35
	// HinkalPrivateSendVariableRate is the variable fee rate (basis points) for private transfers.
	HinkalPrivateSendVariableRate int64 = 5
	// PaySendVariableRate is the variable fee rate (basis points) for public deposit-and-withdraw sends.
	PaySendVariableRate int64 = 10

	// TronDefaultFeeLimitSun is the default fee limit (in SUN) for Tron contract calls.
	TronDefaultFeeLimitSun int64 = 1_000_000_000
	// TronFeePaddingBps is the buffer (basis points) added over the estimated Tron fee.
	TronFeePaddingBps int64 = 2_000
)
