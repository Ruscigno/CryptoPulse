package tx

import (
"fmt"
"math"
"math/big"
"math/rand"
"time"

"go.uber.org/zap"
)

// PlaceOrderMessage represents a place order message for dYdX
type PlaceOrderMessage struct {
Sender        string
Market        string
Side          string
Quantums      uint64
Subticks      uint64
GoodTilBlock  uint32
TimeInForce   string
ClientOrderID uint64
SubaccountID  uint32
}

// CancelOrderMessage represents a cancel order message for dYdX
type CancelOrderMessage struct {
Sender        string
Market        string
ClientOrderID uint64
GoodTilBlock  uint32
SubaccountID  uint32
}

// MessageBuilder handles construction of dYdX protocol messages
type MessageBuilder struct {
logger *zap.Logger
}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder(logger *zap.Logger) *MessageBuilder {
return &MessageBuilder{
logger: logger,
}
}

// BuildPlaceOrderMsg constructs a PlaceOrderMessage for dYdX
func (mb *MessageBuilder) BuildPlaceOrderMsg(
senderAddress string,
market string,
side string,
quantums uint64,
subticks uint64,
goodTilBlock uint32,
timeInForce string,
clientOrderID uint64,
subaccountID uint32,
) (*PlaceOrderMessage, error) {
mb.logger.Debug("Building PlaceOrderMessage",
zap.String("market", market),
zap.String("side", side),
zap.Uint64("quantums", quantums),
zap.Uint64("subticks", subticks))

switch side {
case "BUY", "SELL":
default:
return nil, fmt.Errorf("invalid order side: %s", side)
}

switch timeInForce {
case "GTT", "FOK", "IOC":
default:
timeInForce = "GTT"
}

msg := &PlaceOrderMessage{
Sender:        senderAddress,
Market:        market,
Side:          side,
Quantums:      quantums,
Subticks:      subticks,
GoodTilBlock:  goodTilBlock,
TimeInForce:   timeInForce,
ClientOrderID: clientOrderID,
SubaccountID:  subaccountID,
}

mb.logger.Debug("PlaceOrderMessage built successfully",
zap.String("sender", senderAddress),
zap.Uint64("client_id", clientOrderID))

return msg, nil
}

// BuildCancelOrderMsg constructs a CancelOrderMessage for dYdX
func (mb *MessageBuilder) BuildCancelOrderMsg(
senderAddress string,
market string,
clientOrderID uint64,
goodTilBlock uint32,
subaccountID uint32,
) (*CancelOrderMessage, error) {
mb.logger.Debug("Building CancelOrderMessage",
zap.String("sender", senderAddress),
zap.String("market", market),
zap.Uint64("client_id", clientOrderID))

msg := &CancelOrderMessage{
Sender:        senderAddress,
Market:        market,
ClientOrderID: clientOrderID,
GoodTilBlock:  goodTilBlock,
SubaccountID:  subaccountID,
}

mb.logger.Debug("CancelOrderMessage built successfully",
zap.String("sender", senderAddress))

return msg, nil
}

// GenerateClientOrderID generates a unique client order ID
func (mb *MessageBuilder) GenerateClientOrderID() uint64 {
timestamp := uint64(time.Now().UnixNano() % 1e9)
random := uint64(rand.Uint32())
clientOrderID := (timestamp << 32) | random

mb.logger.Debug("Generated client order ID",
zap.Uint64("client_order_id", clientOrderID))

return clientOrderID
}

// QuantizeSize converts a size to quantums using market configuration
func (mb *MessageBuilder) QuantizeSize(
size float64,
atomicResolution int32,
) (uint64, error) {
mb.logger.Debug("Quantizing size",
zap.Float64("size", size),
zap.Int32("atomic_resolution", atomicResolution))

if size <= 0 {
return 0, fmt.Errorf("size must be positive")
}

divisor := math.Pow(10, float64(-atomicResolution))
quantumsFloat := size / divisor
quantums := uint64(quantumsFloat)

mb.logger.Debug("Size quantized successfully",
zap.Float64("size", size),
zap.Uint64("quantums", quantums))

return quantums, nil
}

// QuantizePrice converts a price to subticks using market configuration
func (mb *MessageBuilder) QuantizePrice(
price float64,
subticksPerTick int32,
) (uint64, error) {
mb.logger.Debug("Quantizing price",
zap.Float64("price", price),
zap.Int32("subticks_per_tick", subticksPerTick))

if price <= 0 {
return 0, fmt.Errorf("price must be positive")
}

divisor := math.Pow(10, float64(-subticksPerTick))
subticksBigFloat := new(big.Float).SetFloat64(price)
divisorBigFloat := new(big.Float).SetFloat64(divisor)
result := new(big.Float).Quo(subticksBigFloat, divisorBigFloat)

subticks, _ := result.Uint64()

mb.logger.Debug("Price quantized successfully",
zap.Float64("price", price),
zap.Uint64("subticks", subticks))

return subticks, nil
}

// ValidateQuantization validates that quantized values are correct
func (mb *MessageBuilder) ValidateQuantization(
quantums uint64,
subticks uint64,
stepBaseQuantums uint64,
) error {
mb.logger.Debug("Validating quantization",
zap.Uint64("quantums", quantums),
zap.Uint64("subticks", subticks),
zap.Uint64("step_base_quantums", stepBaseQuantums))

if quantums%stepBaseQuantums != 0 {
return fmt.Errorf(
"quantums (%d) must be a multiple of stepBaseQuantums (%d)",
quantums,
stepBaseQuantums,
)
}

mb.logger.Debug("Quantization validation passed")
return nil
}
