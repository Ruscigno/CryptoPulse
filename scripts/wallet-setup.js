import { BECH32_PREFIX, LocalWallet } from '@dydxprotocol/v4-client-js';
import { SubaccountInfo } from '@dydxprotocol/v4-client-js';
 
const mnemonic = require('fs').readFileSync('mnemonic.txt', 'utf8').trim();
 
const wallet = await LocalWallet.fromMnemonic(mnemonic, BECH32_PREFIX);

const subaccount = new SubaccountInfo(wallet, 0);