#!/usr/bin/env node

const { FaucetClient } = require("@dydxprotocol/v4-client-js");

// Configuration
const FAUCET_URL = process.env.DYDX_FAUCET_URL || 'https://faucet.v4testnet.dydx.exchange';
const DEFAULT_AMOUNT = 2000; // USDC units
const DEFAULT_SUBACCOUNT = 0;

// Colors for console output
const colors = {
    reset: '\x1b[0m',
    bright: '\x1b[1m',
    red: '\x1b[31m',
    green: '\x1b[32m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m',
    cyan: '\x1b[36m'
};

function log(message, color = colors.reset) {
    console.log(`${color}${message}${colors.reset}`);
}

function getAddressFromEnv() {
    // Try to get address from environment variable
    if (process.env.DYDX_ADDRESS) {
        return process.env.DYDX_ADDRESS;
    }

    // Try to get address from command line argument
    if (process.argv[2]) {
        return process.argv[2];
    }

    // Try to derive from mnemonic if available
    if (process.env.MNEMONIC) {
        log('⚠️  MNEMONIC found but address derivation not implemented in this script', colors.yellow);
        log('   Please provide DYDX_ADDRESS environment variable or command line argument', colors.yellow);
    }

    return null;
}

function printUsage() {
    log('\n📖 dYdX Testnet Faucet Usage:', colors.bright);
    log('');
    log('Environment Variables:', colors.cyan);
    log('  DYDX_ADDRESS     - dYdX testnet address (required)');
    log('  DYDX_FAUCET_URL  - Faucet URL (optional, defaults to official testnet faucet)');
    log('  FAUCET_AMOUNT    - Amount to request in USDC units (optional, default: 2000)');
    log('  FAUCET_SUBACCOUNT - Subaccount number (optional, default: 0)');
    log('');
    log('Command Line Usage:', colors.cyan);
    log('  node obtain-testnet-funds-via-faucet.js [dydx_address]');
    log('');
    log('Examples:', colors.green);
    log('  DYDX_ADDRESS=dydx1abc... node obtain-testnet-funds-via-faucet.js');
    log('  node obtain-testnet-funds-via-faucet.js dydx1abc...');
    log('  DYDX_ADDRESS=dydx1abc... FAUCET_AMOUNT=5000 node obtain-testnet-funds-via-faucet.js');
    log('');
}

async function requestFaucetFunds() {
    try {
        log('🚰 dYdX Testnet Faucet', colors.bright);
        log('='.repeat(50), colors.blue);

        // Get configuration
        const address = getAddressFromEnv();
        const amount = parseInt(process.env.FAUCET_AMOUNT) || DEFAULT_AMOUNT;
        const subaccount = parseInt(process.env.FAUCET_SUBACCOUNT) || DEFAULT_SUBACCOUNT;

        if (!address) {
            log('❌ Error: No dYdX address provided', colors.red);
            printUsage();
            process.exit(1);
        }

        // Validate address format
        if (!address.startsWith('dydx1') || address.length < 20) {
            log('❌ Error: Invalid dYdX address format', colors.red);
            log(`   Expected format: dydx1... (got: ${address})`, colors.red);
            process.exit(1);
        }

        log(`📍 Address: ${address}`, colors.cyan);
        log(`💰 Amount: ${amount} USDC`, colors.cyan);
        log(`🔢 Subaccount: ${subaccount}`, colors.cyan);
        log(`🌐 Faucet URL: ${FAUCET_URL}`, colors.cyan);
        log('');

        // Create faucet client
        log('🔗 Connecting to faucet...', colors.yellow);
        const client = new FaucetClient(FAUCET_URL);

        // Request funds
        log('💸 Requesting testnet funds...', colors.yellow);
        const response = await client.fill(address, subaccount, amount);

        // Handle response
        if (response && response.status === 'success') {
            log('✅ Success! Testnet funds requested successfully', colors.green);
            log(`   Transaction: ${response.txHash || 'N/A'}`, colors.green);
            log(`   Amount: ${amount} USDC`, colors.green);
            log(`   Address: ${address}`, colors.green);
            log(`   Subaccount: ${subaccount}`, colors.green);
        } else {
            log('⚠️  Faucet request completed with unknown status', colors.yellow);
            log(`   Response: ${JSON.stringify(response, null, 2)}`, colors.yellow);
        }

        log('');
        log('📝 Note: It may take a few minutes for funds to appear in your account', colors.blue);
        log('🔍 Check your balance at: https://trade.v4testnet.dydx.exchange/', colors.blue);

    } catch (error) {
        log('❌ Error requesting faucet funds:', colors.red);

        if (error.message.includes('rate limit') || error.message.includes('429')) {
            log('   Rate limit exceeded. Please wait before trying again.', colors.red);
            log('   Faucets typically have cooldown periods between requests.', colors.yellow);
        } else if (error.message.includes('network') || error.message.includes('ENOTFOUND')) {
            log('   Network error. Please check your internet connection.', colors.red);
            log(`   Faucet URL: ${FAUCET_URL}`, colors.yellow);
        } else if (error.message.includes('invalid address')) {
            log('   Invalid address format. Please check your dYdX address.', colors.red);
        } else if (error.message.includes('certificate') || error.message.includes('SSL') || error.message.includes('TLS')) {
            log('   SSL certificate error with the faucet endpoint.', colors.red);
            log('   This is a known issue with the dYdX testnet faucet.', colors.yellow);
        } else {
            log(`   ${error.message}`, colors.red);
            if (error.response) {
                log(`   HTTP Status: ${error.response.status}`, colors.red);
                log(`   Response: ${JSON.stringify(error.response.data, null, 2)}`, colors.red);
            }
        }

        log('');
        log('💡 Troubleshooting:', colors.cyan);
        log('   1. Verify your dYdX testnet address is correct');
        log('   2. Check if you have exceeded the faucet rate limit');
        log('   3. Ensure you have internet connectivity');
        log('   4. Try again in a few minutes');

        // Additional troubleshooting for SSL certificate errors
        if (error.message.includes('certificate') || error.message.includes('SSL') || error.message.includes('TLS')) {
            log('');
            log('🔧 SSL Certificate Issue Solutions:', colors.cyan);
            log('   1. Use the web faucet directly:', colors.yellow);
            log('      https://faucet.v4testnet.dydx.exchange/', colors.blue);
            log('   2. Try using curl with --insecure flag:', colors.yellow);
            log(`      curl -k -X POST "${FAUCET_URL}/fill" \\`, colors.blue);
            log(`           -H "Content-Type: application/json" \\`, colors.blue);
            log(`           -d '{"address":"YOUR_ADDRESS","subaccountNumber":0,"amount":2000}'`, colors.blue);
            log('   3. Use the dYdX Discord faucet bot:', colors.yellow);
            log('      https://discord.gg/dydx', colors.blue);
            log('   4. Contact dYdX support if the issue persists', colors.yellow);
        }

        process.exit(1);
    }
}

// Main execution function
async function main() {
    // Handle command line help
    if (process.argv.includes('--help') || process.argv.includes('-h')) {
        printUsage();
        process.exit(0);
    }

    // Run the faucet request
    await requestFaucetFunds();
}

// Run the main function
main().catch((error) => {
    console.error('Unexpected error:', error);
    process.exit(1);
});