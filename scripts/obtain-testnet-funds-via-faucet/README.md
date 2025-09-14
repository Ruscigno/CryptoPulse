# dYdX Testnet Faucet Script

This script allows you to request testnet funds from the dYdX V4 testnet faucet programmatically.

## Features

- ✅ **Easy to use**: Simple command-line interface
- ✅ **Configurable**: Environment variables and command-line options
- ✅ **Error handling**: Comprehensive error messages and troubleshooting
- ✅ **Colorized output**: Clear visual feedback
- ✅ **Make integration**: Convenient make commands

## Installation

### Option 1: Using Make (Recommended)
```bash
# From project root
make faucet-install
```

### Option 2: Manual Installation
```bash
cd scripts/obtain-testnet-funds-via-faucet
npm install
```

## Usage

### Using Make Commands (Recommended)

#### Show Help
```bash
make faucet-help
```

#### Request Funds
```bash
# Add your dYdX address to .env.local (one-time setup)
echo "DYDX_ADDRESS=dydx1your_address_here" >> .env.local

# Request funds (reads address from .env.local)
make faucet

# Check current address
make faucet-address

# Override address for one-time use
make faucet DYDX_ADDRESS=dydx1different_address

# Or set environment variable first
export DYDX_ADDRESS=dydx1your_address_here
make faucet
```

### Direct Node.js Usage

#### Show Help
```bash
node obtain-testnet-funds-via-faucet.js --help
```

#### Request Funds
```bash
# Using environment variable
DYDX_ADDRESS=dydx1your_address_here node obtain-testnet-funds-via-faucet.js

# Using command line argument
node obtain-testnet-funds-via-faucet.js dydx1your_address_here

# With custom amount
DYDX_ADDRESS=dydx1your_address_here FAUCET_AMOUNT=5000 node obtain-testnet-funds-via-faucet.js
```

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DYDX_ADDRESS` | Your dYdX testnet address | - | ✅ Yes |
| `DYDX_FAUCET_URL` | Faucet endpoint URL | `https://faucet.v4testnet.dydx.exchange` | No |
| `FAUCET_AMOUNT` | Amount to request (USDC units) | `2000` | No |
| `FAUCET_SUBACCOUNT` | Subaccount number | `0` | No |

### Getting Your dYdX Address

1. **From Wallet**: If you have a dYdX wallet, copy your testnet address
2. **From Mnemonic**: Use dYdX tools to derive address from mnemonic
3. **Create New**: Visit [dYdX Testnet](https://trade.v4testnet.dydx.exchange/) to create a new wallet

## Examples

### Basic Usage
```bash
# Request 2000 USDC (default amount)
make faucet DYDX_ADDRESS=dydx1abc123...

# Request custom amount
make faucet DYDX_ADDRESS=dydx1abc123... FAUCET_AMOUNT=5000

# Use different subaccount
make faucet DYDX_ADDRESS=dydx1abc123... FAUCET_SUBACCOUNT=1
```

### Integration with CryptoPulse
```bash
# Use the same address as your application
export DYDX_ADDRESS=$(grep DYDX_ADDRESS .env | cut -d '=' -f2)
make faucet
```

## Output Examples

### Successful Request
```
🚰 dYdX Testnet Faucet
==================================================
📍 Address: dydx1abc123...
💰 Amount: 2000 USDC
🔢 Subaccount: 0
🌐 Faucet URL: https://faucet.v4testnet.dydx.exchange

🔗 Connecting to faucet...
💸 Requesting testnet funds...
✅ Success! Testnet funds requested successfully
   Transaction: 0x123abc...
   Amount: 2000 USDC
   Address: dydx1abc123...
   Subaccount: 0

📝 Note: It may take a few minutes for funds to appear in your account
🔍 Check your balance at: https://trade.v4testnet.dydx.exchange/
```

### Error Handling
```
❌ Error requesting faucet funds:
   Rate limit exceeded. Please wait before trying again.
   Faucets typically have cooldown periods between requests.

💡 Troubleshooting:
   1. Verify your dYdX testnet address is correct
   2. Check if you have exceeded the faucet rate limit
   3. Ensure you have internet connectivity
   4. Try again in a few minutes
```

## Troubleshooting

### Common Issues

#### 1. "No dYdX address provided"
**Solution**: Set the `DYDX_ADDRESS` environment variable or provide address as argument

#### 2. "Invalid dYdX address format"
**Solution**: Ensure your address starts with `dydx1` and is the correct length

#### 3. "Rate limit exceeded"
**Solution**: Wait for the cooldown period (usually 24 hours) before requesting again

#### 4. "Network error"
**Solution**: Check internet connection and faucet URL accessibility

#### 5. "Certificate has expired"
**Solution**: The faucet SSL certificate may be expired. Try again later or contact dYdX support

### Debugging

#### Enable Verbose Output
```bash
# Add debugging to see more details
DEBUG=* make faucet DYDX_ADDRESS=dydx1abc123...
```

#### Check Network Connectivity
```bash
# Test faucet URL accessibility
curl -I https://faucet.v4testnet.dydx.exchange
```

#### Verify Address Format
```bash
# Your address should look like this
echo $DYDX_ADDRESS
# Output: dydx1abc123def456ghi789jkl012mno345pqr678stu
```

## Development

### Dependencies
- Node.js >= 18.0.0
- `@dydxprotocol/v4-client-js` ^3.0.3

### File Structure
```
scripts/obtain-testnet-funds-via-faucet/
├── obtain-testnet-funds-via-faucet.js  # Main script
├── package.json                        # Node.js dependencies
├── pnpm-lock.yaml                     # Lock file
├── node_modules/                      # Dependencies (gitignored)
└── README.md                          # This file
```

### Contributing
1. Test changes with various address formats
2. Ensure error handling covers edge cases
3. Update documentation for new features
4. Test both make commands and direct node usage

## Security Notes

- ⚠️ **Testnet Only**: This script is for testnet funds only
- ⚠️ **Rate Limits**: Respect faucet rate limits to avoid being blocked
- ⚠️ **Address Validation**: Always verify addresses before requesting funds
- ⚠️ **Network Security**: Use secure networks when requesting funds

## Support

- **dYdX Documentation**: [docs.dydx.exchange](https://docs.dydx.exchange)
- **dYdX Discord**: [discord.gg/dydx](https://discord.gg/dydx)
- **GitHub Issues**: Report bugs in the CryptoPulse repository
