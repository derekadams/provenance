package handlers

import (
	"fmt"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	msgbasedfeetypes "github.com/provenance-io/provenance/x/msgfees/types"
)

type MsgBasedFeeInvoker struct {
	msgBasedFeeKeeper msgbasedfeetypes.MsgBasedFeeKeeper
	bankKeeper        msgbasedfeetypes.BankKeeper
	accountKeeper     msgbasedfeetypes.AccountKeeper
	feegrantKeeper    msgbasedfeetypes.FeegrantKeeper
	txDecoder         sdk.TxDecoder
}

// NewMsgBasedFeeInvoker concrete impl of how to charge Msg Based Fees
func NewMsgBasedFeeInvoker(bankKeeper msgbasedfeetypes.BankKeeper, accountKeeper msgbasedfeetypes.AccountKeeper,
	feegrantKeeper msgbasedfeetypes.FeegrantKeeper, msgBasedFeeKeeper msgbasedfeetypes.MsgBasedFeeKeeper, decoder sdk.TxDecoder) MsgBasedFeeInvoker {
	return MsgBasedFeeInvoker{
		msgBasedFeeKeeper,
		bankKeeper,
		accountKeeper,
		feegrantKeeper,
		decoder,
	}
}

func (afd MsgBasedFeeInvoker) Invoke(ctx sdk.Context, simulate bool) (coins sdk.Coins, err error) {
	chargedFees := sdk.Coins{}

	if ctx.TxBytes() != nil && len(ctx.TxBytes()) != 0 {
		ctx.Logger().Debug("In chargeFees()")
		originalGasMeter := ctx.GasMeter()
		// eat up the gas cost for charging fees.
		ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter())

		var msgs []sdk.Msg
		tx, err := afd.txDecoder(ctx.TxBytes())
		if err != nil {
			panic(fmt.Errorf("error in chargeFees() while getting txBytes: %w", err))
		}
		msgs = tx.GetMsgs()

		// cast to FeeTx
		feeTx, ok := tx.(sdk.FeeTx)
		// only charge additional fee if of type FeeTx since it should give fee payer.
		// for provenance should be a FeeTx since antehandler should enforce it, but
		// not adding complexity here
		if ok {

			feePayer := feeTx.FeePayer()
			feeGranter := feeTx.FeeGranter()
			deductFeesFrom := feePayer
			// if fee granter set deduct fee from feegranter account.
			// this works with only when feegrant enabled.


			deductFeesFromAcc := afd.accountKeeper.GetAccount(ctx, deductFeesFrom)
			if deductFeesFromAcc == nil {
				panic("fee payer address: %s does not exist")
			}

			chargedFees = make(sdk.Coins, len(msgs))

			for _, msg := range msgs {
				ctx.Logger().Debug(fmt.Sprintf("The message type in defer block for fee charging : %s", sdk.MsgTypeURL(msg)))
				msgFees, err := afd.msgBasedFeeKeeper.GetMsgBasedFee(ctx, sdk.MsgTypeURL(msg))
				if err != nil {
					// do nothing for now
					ctx.Logger().Error("unable to get message fees", "err", err)
				}
				if msgFees != nil {
					ctx.Logger().Info("Retrieved a msg based fee.")
						if feeGranter != nil {
							if afd.feegrantKeeper == nil {
								return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "fee grants are not enabled")
							} else if !feeGranter.Equals(feePayer) {
								err := afd.feegrantKeeper.UseGrantedFees(ctx, feeGranter, feePayer, sdk.Coins{msgFees.AdditionalFee}, tx.GetMsgs())
								if err != nil {
									return nil, sdkerrors.Wrapf(err, "%s not allowed to pay fees from %s", feeGranter, feePayer)
								}
							}

							deductFeesFrom = feeGranter
						}
						err = afd.msgBasedFeeKeeper.DeductFees(afd.bankKeeper, ctx, deductFeesFromAcc, sdk.Coins{msgFees.AdditionalFee})
						if err != nil {
							return nil, err
						}
					chargedFees = sdk.Coins{msgFees.AdditionalFee}
				}
			}
			// set back the original gasMeter
			ctx = ctx.WithGasMeter(originalGasMeter)
		}
	}
	return chargedFees, nil
}
