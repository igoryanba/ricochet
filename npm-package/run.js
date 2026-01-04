#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

const BIN_DIR = path.join(__dirname, 'bin');
const BIN_NAME = process.platform === 'win32' ? 'ricochet.exe' : 'ricochet';
const BIN_PATH = path.join(BIN_DIR, BIN_NAME);

// Check if binary exists
if (!fs.existsSync(BIN_PATH)) {
    console.error('❌ Ricochet binary not found. Running installer...');
    require('./install.js');

    // Check again after install
    if (!fs.existsSync(BIN_PATH)) {
        console.error('❌ Installation failed. Please install manually.');
        process.exit(1);
    }
}

// Pass through all arguments and environment
const child = spawn(BIN_PATH, process.argv.slice(2), {
    stdio: 'inherit',
    env: process.env
});

child.on('error', (err) => {
    console.error('Failed to start Ricochet:', err.message);
    process.exit(1);
});

child.on('exit', (code) => {
    process.exit(code || 0);
});
