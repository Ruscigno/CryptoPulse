#!/usr/bin/env node

/**
 * Script to derive dYdX address from mnemonic
 * This helps you get your real dYdX testnet address for wallet setup
 */

const crypto = require('crypto');
const { execSync } = require('child_process');

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

async function deriveDydxAddress() {
    try {
        // Check if we have the dYdX client available
        try {
            require('@dydxprotocol/v4-client-js');
        } catch (error) {
            log('❌ Error: @dydxprotocol/v4-client-js not found', colors.red);
            log('Please run: cd scripts/obtain-testnet-funds-via-faucet && npm install', colors.yellow);
            process.exit(1);
        }

        const { LocalWallet } = require('@dydxprotocol/v4-client-js');
        const { BECH32_PREFIX } = require('@dydxprotocol/v4-client-js');

        // Get mnemonic from environment or .env.local
        let mnemonic = process.env.MNEMONIC;
        
        if (!mnemonic) {
            // Try to read from .env.local
            const fs = require('fs');
            const path = require('path');
            
            const envPath = path.join(__dirname, '..', '.env.local');
            if (fs.existsSync(envPath)) {
                const envContent = fs.readFileSync(envPath, 'utf8');
                const mnemonicMatch = envContent.match(/MNEMONIC="([^"]+)"/);
                if (mnemonicMatch) {
                    mnemonic = mnemonicMatch[1];
                }
            }
        }

        if (!mnemonic) {
            log('❌ Error: MNEMONIC not found', colors.red);
            log('Please set MNEMONIC in .env.local or as environment variable', colors.yellow);
            process.exit(1);
        }

        log('🔑 Deriving dYdX address from mnemonic...', colors.cyan);
        log('');

        // Create wallet from mnemonic
        const wallet = await LocalWallet.fromMnemonic(mnemonic, BECH32_PREFIX);
        
        log('✅ Successfully derived dYdX address!', colors.green);
        log('');
        log('📍 Your dYdX Testnet Address:', colors.cyan);
        log(`   ${wallet.address}`, colors.yellow);
        log('');
        log('🔧 Next Steps:', colors.cyan);
        log('1. Update your .env.local file:', colors.yellow);
        log(`   DYDX_ADDRESS=${wallet.address}`, colors.blue);
        log('');
        log('2. Initialize your wallet by connecting to dYdX web interface:', colors.yellow);
        log('   https://v4.testnet.dydx.exchange/', colors.blue);
        log('');
        log('3. Request testnet funds:', colors.yellow);
        log('   make faucet', colors.blue);
        log('');
        log('4. Check wallet status:', colors.yellow);
        log('   make dydx-check-wallet', colors.blue);

        // Automatically update .env.local if possible
        try {
            const fs = require('fs');
            const path = require('path');
            
            const envPath = path.join(__dirname, '..', '.env.local');
            if (fs.existsSync(envPath)) {
                let envContent = fs.readFileSync(envPath, 'utf8');
                
                // Replace the placeholder address
                const oldAddress = 'dydx1abc123456789abcdefghijklmnopqrstuvwxyz';
                if (envContent.includes(oldAddress)) {
                    envContent = envContent.replace(oldAddress, wallet.address);
                    fs.writeFileSync(envPath, envContent);
                    log('');
                    log('✅ Automatically updated .env.local with your real address!', colors.green);
                }
            }
        } catch (error) {
            log('');
            log('⚠️  Could not automatically update .env.local', colors.yellow);
            log('   Please manually update DYDX_ADDRESS in .env.local', colors.yellow);
        }

    } catch (error) {
        log('❌ Error deriving address:', colors.red);
        log(`   ${error.message}`, colors.red);
        
        if (error.message.includes('Invalid mnemonic')) {
            log('');
            log('💡 Troubleshooting:', colors.cyan);
            log('   1. Verify your mnemonic has 12 or 24 words', colors.yellow);
            log('   2. Check for typos in the mnemonic phrase', colors.yellow);
            log('   3. Ensure words are separated by single spaces', colors.yellow);
        }
        
        process.exit(1);
    }
}

// Handle command line help
if (process.argv.includes('--help') || process.argv.includes('-h')) {
    log('🔑 dYdX Address Derivation Tool', colors.cyan);
    log('');
    log('Derives your dYdX testnet address from your mnemonic phrase.', colors.yellow);
    log('');
    log('Usage:', colors.cyan);
    log('  node derive-dydx-address.js', colors.blue);
    log('  MNEMONIC="your mnemonic here" node derive-dydx-address.js', colors.blue);
    log('');
    log('The script will:', colors.cyan);
    log('  1. Read mnemonic from .env.local or environment variable', colors.yellow);
    log('  2. Derive your dYdX testnet address', colors.yellow);
    log('  3. Automatically update .env.local with the real address', colors.yellow);
    log('  4. Provide next steps for wallet initialization', colors.yellow);
    process.exit(0);
}

// Main execution
deriveDydxAddress();
