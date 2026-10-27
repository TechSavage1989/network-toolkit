'use strict';
// Retired deliberately: the old live Linux fixture required plaintext vault
// fallback and a saved-secret GET endpoint, neither of which is supported now.
// Do not restore either behavior to make this legacy entry point pass.
console.error([
 'RETIRED: connection-settings.browser.cjs cannot verify the secured vault.',
 'Run node scripts/credential-security.browser.cjs for isolated, synthetic UI coverage.',
 'That route fixture does NOT verify Windows DPAPI, native approval or native bootstrap.',
 'Actual Windows security verification is a separate required release gate; see docs/CREDENTIAL_SECURITY.md.'
].join('\n'));
process.exitCode=1;
