# MCPHost Script Examples

This directory contains example scripts demonstrating various features of MCPHost's script mode.

## Scripts

### `default-values-demo.sh`
Demonstrates the new default values feature for script variables.

**Features showcased:**
- Optional variables with default values using `${var:-default}` syntax
- Mixed required and optional variables
- Default values in MCP server configuration
- Complex default values (paths, commands, formats)

**Usage:**
```bash
# Use all defaults
mcphost script default-values-demo.sh

# Override specific variables
mcphost script default-values-demo.sh --args:user_name "John" --args:work_dir "/projects"

# Override multiple variables
mcphost script default-values-demo.sh \
  --args:user_name "Alice" \
  --args:editor "vim" \
  --args:format "json"
```

### `tls-test-script.sh`
Demonstrates TLS skip verify for connecting to providers with self-signed certificates.

**Features showcased:**
- `tls-skip-verify` configuration in script frontmatter
- Connecting to HTTPS endpoints with self-signed certificates
- Security considerations for development environments

**Usage:**
```bash
# Run with TLS skip verify enabled (configured in script)
mcphost script tls-test-script.sh

# Override the provider URL
mcphost script tls-test-script.sh --provider-url https://192.168.1.100:443

# Disable TLS skip verify via command line (overrides script config)
mcphost script tls-test-script.sh --tls-skip-verify=false
```

⚠️ **WARNING**: Only use `tls-skip-verify` for development or when connecting to trusted servers with self-signed certificates.

## Variable Syntax Reference

MCPHost scripts support two types of variables:

### Required Variables
```bash
${variable}
```
- Must be provided via `--args:variable value`
- Script will fail if not provided

### Optional Variables with Defaults
```bash
${variable:-default_value}
```
- Uses `default_value` if not provided
- Can be overridden with `--args:variable value`
- Supports empty defaults: `${var:-}`
- Supports complex defaults: `${path:-/tmp/default/path}`

## Best Practices

1. **Use descriptive variable names**: `${user_name}` instead of `${name}`
2. **Provide sensible defaults**: Choose defaults that work in most environments
3. **Document variables**: Include usage examples in script comments
4. **Mix required and optional**: Use required variables for critical inputs, optional for preferences
5. **Test with defaults**: Ensure scripts work with all default values

## Backward Compatibility

All existing scripts using `${variable}` syntax continue to work unchanged. The new default syntax is purely additive.