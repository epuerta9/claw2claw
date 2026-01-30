# claw2claw Claude Extension

## Commands

### /share <file>
Share a file securely with another Claude user.

Usage:
```
/share context.md
```

This will:
1. Generate a unique code phrase
2. Wait for the receiver to connect
3. Perform PAKE key exchange
4. Send the encrypted file

Output the code phrase for the user to share with their recipient.

### /receive <code>
Receive a shared file using a code phrase.

Usage:
```
/receive swift-tiger-gold-42
```

This will:
1. Connect to the relay
2. Perform PAKE key exchange
3. Receive and decrypt the file
4. Save to current directory

## Configuration

Set relay URL via environment variable:
```
CLAW_RELAY_URL=wss://relay.claw2claw.io
```

## Security

- All content is end-to-end encrypted
- The relay server cannot see file contents
- Code phrases are never transmitted (only hashes)
- Each transfer uses fresh encryption keys
