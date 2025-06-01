#!/usr/bin/env node
const readline = require('readline');
const crypto = require('crypto');
const fs = require('fs');
const { execSync } = require('child_process');

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

class N8nDeployer {
  constructor() {
    this.config = {};
  }

  // Generate secure random password
  generatePassword(length = 24) {
    const charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*';
    let password = '';
    for (let i = 0; i < length; i++) {
      password += charset.charAt(Math.floor(Math.random() * charset.length));
    }
    return password;
  }

  // Validate password strength  
  validatePassword(password) {
    const minLength = 8;
    const hasUpper = /[A-Z]/.test(password);
    const hasLower = /[a-z]/.test(password);
    const hasNumber = /\d/.test(password);
    const hasSpecial = /[!@#$%^&*]/.test(password);

    if (password.length < minLength) {
      return `Password must be at least ${minLength} characters long`;
    }
    if (!hasUpper) return 'Password must contain uppercase letters';
    if (!hasLower) return 'Password must contain lowercase letters';
    if (!hasNumber) return 'Password must contain numbers';
    
    return null; // Valid password
  }

  // Prompt user for input
  async prompt(question, options = {}) {
    return new Promise((resolve) => {
      const { secret = false, validator = null } = options;
      
      if (secret) {
        // Hide input for passwords
        process.stdout.write(question);
        process.stdin.setRawMode(true);
        process.stdin.resume();
        process.stdin.setEncoding('utf8');
        
        let input = '';
        process.stdin.on('data', (char) => {
          if (char === '\u0003') process.exit(); // Ctrl+C
          if (char === '\r' || char === '\n') {
            process.stdin.setRawMode(false);
            process.stdin.pause();
            process.stdout.write('\n');
            resolve(input);
          } else if (char === '\u007f') { // Backspace
            if (input.length > 0) {
              input = input.slice(0, -1);
              process.stdout.write('\b \b');
            }
          } else {
            input += char;
            process.stdout.write('*');
          }
        });
      } else {
        rl.question(question, (answer) => {
          if (validator) {
            const error = validator(answer);
            if (error) {
              console.log(`❌ ${error}`);
              return this.prompt(question, options).then(resolve);
            }
          }
          resolve(answer || options.default);
        });
      }
    });
  }

  // Setup database password
  async setupDatabasePassword() {
    console.log('\n🔐 PostgreSQL Database Password Setup');
    console.log('1. Generate random secure password (recommended)');
    console.log('2. Enter custom password');
    console.log('3. Use existing environment variable');
    
    const choice = await this.prompt('Choose option (1-3): ', { default: '1' });
    
    switch (choice) {
      case '1':
        this.config.dbPassword = this.generatePassword();
        console.log('✅ Generated secure password');
        break;
        
      case '2':
        while (true) {
          const password = await this.prompt('Enter PostgreSQL password: ', { secret: true });
          const confirm = await this.prompt('Confirm password: ', { secret: true });
          
          if (password !== confirm) {
            console.log('❌ Passwords don\'t match. Try again.');
            continue;
          }
          
          const error = this.validatePassword(password);
          if (error) {
            console.log(`❌ ${error}`);
            continue;
          }
          
          this.config.dbPassword = password;
          console.log('✅ Password accepted');
          break;
        }
        break;
        
      case '3':
        this.config.dbPassword = process.env.DB_PASSWORD || this.generatePassword();
        if (!process.env.DB_PASSWORD) {
          console.log('✅ Generated new password (DB_PASSWORD not found in environment)');
        } else {
          console.log('✅ Using existing DB_PASSWORD from environment');
        }
        break;
        
      default:
        this.config.dbPassword = this.generatePassword();
        console.log('✅ Using generated password (invalid choice)');
    }
  }

  // Setup n8n admin credentials
  async setupN8nCredentials() {
    console.log('\n👤 n8n Admin Credentials Setup');
    
    this.config.n8nUser = await this.prompt('Enter n8n admin username: ', { default: 'admin' });
    
    while (true) {
      const password = await this.prompt('Enter n8n admin password: ', { secret: true });
      const confirm = await this.prompt('Confirm n8n password: ', { secret: true });
      
      if (password !== confirm) {
        console.log('❌ Passwords don\'t match. Try again.');
        continue;
      }
      
      const error = this.validatePassword(password);
      if (error) {
        console.log(`❌ ${error}`);
        continue;
      }
      
      this.config.n8nPassword = password;
      console.log('✅ n8n credentials accepted');
      break;
    }
  }

  // Setup app configuration
  async setupAppConfig() {
    console.log('\n🚀 App Configuration');
    
    this.config.appName = await this.prompt('Enter your Zeabur app name: ', { default: 'n8n-app' });
    this.config.appUrl = `https://${this.config.appName}.zeabur.app`;
    
    console.log(`✅ App URL will be: ${this.config.appUrl}`);
  }

  // Generate configuration files
  generateFiles() {
    console.log('\n📝 Generating configuration files...');
    
    // Generate .env file
    const envContent = `# Generated on ${new Date().toISOString()}
# PostgreSQL Configuration
DB_PASSWORD=${this.config.dbPassword}
POSTGRES_PASSWORD=${this.config.dbPassword}

# n8n Configuration  
N8N_BASIC_AUTH_USER=${this.config.n8nUser}
N8N_BASIC_AUTH_PASSWORD=${this.config.n8nPassword}

# App Configuration
APP_NAME=${this.config.appName}
ZEABUR_WEB_URL=${this.config.appUrl}

# Security
JWT_SECRET=${this.generatePassword()}
ENCRYPTION_KEY=${crypto.randomBytes(32).toString('hex')}

# Additional Settings
GENERIC_TIMEZONE=UTC
EXECUTIONS_PROCESS=main
N8N_METRICS=true
N8N_LOG_LEVEL=info
`;

    fs.writeFileSync('.env', envContent);
    
    // Generate deployment script
    const deployScript = `#!/bin/bash
# Auto-generated deployment script

echo "🚀 Setting up n8n deployment..."

# Set environment variables in Zeabur
zeabur env set DB_POSTGRESDB_PASSWORD="${this.config.dbPassword}"
zeabur env set POSTGRES_PASSWORD="${this.config.dbPassword}"
zeabur env set N8N_BASIC_AUTH_USER="${this.config.n8nUser}"
zeabur env set N8N_BASIC_AUTH_PASSWORD="${this.config.n8nPassword}"
zeabur env set WEBHOOK_URL="${this.config.appUrl}/"
zeabur env set N8N_EDITOR_BASE_URL="${this.config.appUrl}/"

echo "✅ Environment variables configured!"
echo "🚀 Deploying to Zeabur..."

zeabur deploy

echo "✅ Deployment complete!"
echo "🌐 Your n8n instance: ${this.config.appUrl}"
echo "👤 Login: ${this.config.n8nUser}"
echo "🔑 Password: [check .env file]"
`;

    fs.writeFileSync('deploy.sh', deployScript);
    fs.chmodSync('deploy.sh', '755');
    
    console.log('✅ Generated files:');
    console.log('  - .env (environment variables)');
    console.log('  - deploy.sh (deployment script)');
  }

  // Main deployment flow
  async deploy() {
    console.log('🚀 n8n Zeabur Deployment Setup');
    console.log('================================');
    
    try {
      await this.setupDatabasePassword();
      await this.setupN8nCredentials();
      await this.setupAppConfig();
      
      this.generateFiles();
      
      console.log('\n📋 Deployment Summary:');
      console.log(`- App Name: ${this.config.appName}`);
      console.log(`- App URL: ${this.config.appUrl}`);
      console.log(`- Admin User: ${this.config.n8nUser}`);
      console.log('- Passwords: Stored securely in .env file');
      
      console.log('\n🚀 Next Steps:');
      console.log('1. Run: ./deploy.sh (if you have Zeabur CLI installed)');
      console.log('2. Or manually set environment variables in Zeabur dashboard');
      console.log('3. Push your code and deploy');
      
      console.log('\n🔒 Security Reminder:');
      console.log('- Keep your .env file secure');
      console.log('- Add .env to .gitignore');
      console.log('- Use strong passwords');
      
    } catch (error) {
      console.error('❌ Deployment setup failed:', error.message);
    } finally {
      rl.close();
    }
  }
}

// Run the deployment setup
if (require.main === module) {
  const deployer = new N8nDeployer();
  deployer.deploy();
}

module.exports = N8nDeployer;