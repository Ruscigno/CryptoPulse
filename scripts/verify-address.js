#!/usr/bin/env node

/**
 * Address Verification Tool
 * 
 * This helps verify if your derived address matches what you see
 * in the dYdX web interface.
 */

const fs = require('fs');
const path = require('path');

// Colors for console output
const colors = {
    red: '\x1b[31m',
    green: '\x1b[32m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m',
    cyan: '\x1b[36m',
    reset: '\x1b[0m'
};

function log(message, color = colors.reset) {
    console.log(`${color}${message}${colors.reset}`);
}

async function main() {
    try {
        log('🔍 dYdX Address Verification Tool', colors.cyan);
        log('==================================', colors.cyan);
        log('');

        // Read current address from .env.local
        const envPath = path.join(__dirname, '..', '.env.local');
        
        if (!fs.existsSync(envPath)) {
            throw new Error('.env.local file not found');
        }

        const envContent = fs.readFileSync(envPath, 'utf8');
        
        // Extract current address
        const addressMatch = envContent.match(/DYDX_ADDRESS=([^\s]+)/);
        if (!addressMatch) {
            throw new Error('DYDX_ADDRESS not found in .env.local');
        }

        const currentAddress = addressMatch[1];
        log('📍 Current Address in .env.local:', colors.green);
        log(`   ${currentAddress}`, colors.yellow);
        log('');

        // Instructions for verification
        log('🔧 Verification Steps:', colors.cyan);
        log('1. Open: https://v4.testnet.dydx.exchange/', colors.yellow);
        log('2. Connect your wallet using your mnemonic', colors.yellow);
        log('3. Look for your address in the interface (usually top-right)', colors.yellow);
        log('4. Compare it with the address above', colors.yellow);
        log('');

        log('❓ Address Comparison:', colors.cyan);
        log('• If addresses MATCH: Your configuration is correct', colors.green);
        log('• If addresses DIFFER: We need to update .env.local', colors.red);
        log('');

        log('🚀 Next Steps if Addresses Match:', colors.cyan);
        log('1. In the web interface, look for:', colors.yellow);
        log('   • "Get Testnet Funds" button', colors.blue);
        log('   • "Faucet" option', colors.blue);
        log('   • "Deposit" section', colors.blue);
        log('2. Use any of these to create your first subaccount', colors.yellow);
        log('3. Then run: make dydx-check-wallet', colors.yellow);
        log('');

        log('💡 Alternative - Discord Faucet:', colors.cyan);
        log('1. Join: https://discord.gg/dydx', colors.yellow);
        log('2. Find: #testnet-faucet channel', colors.yellow);
        log(`3. Use: /faucet ${currentAddress}`, colors.blue);
        log('');

        log('🔍 Debug Information:', colors.cyan);
        log('• API Endpoint: https://indexer.v4testnet.dydx.exchange/v4', colors.yellow);
        log('• Chain ID: dydx-testnet-4', colors.yellow);
        log('• Address Format: dydx1... (39 characters after dydx1)', colors.yellow);

    } catch (error) {
        log('');
        log(`❌ Error: ${error.message}`, colors.red);
        process.exit(1);
    }
}

main();
