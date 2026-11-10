NETWORK TOOLKIT - DEFAULT TERMINAL TEST (temporary, side-loaded)

What this checks
  Whether Command Prompt and Windows PowerShell open inside Network Toolkit
  when you choose it as the Windows default terminal, and whether you can type
  in them. This is a test build, not a release.

You need
  - Windows 11 version 22H2 or later.
  - Windows Terminal installed (it comes with most Windows 11 PCs).
  Your normal Network Toolkit settings and data are not touched: the test
  window keeps its data in a temporary folder.

1. Extract the zip to a folder. SHA256SUMS.txt lists each file's checksum.

2. Open Windows PowerShell (not as administrator) in that folder and run:
     powershell -ExecutionPolicy Bypass -File .\Install-NetworkToolkitTerminalTest.ps1
   Approve the Windows prompt. It trusts the test certificate on this PC only.
   The certificate's signing key was deleted after signing, so nothing else can
   be signed with it.

3. Open Settings > System > For developers > Terminal and choose
   "Network Toolkit Default Terminal Experiment". (On Windows 11 22H2 the page
   is Settings > Privacy & security > For developers.)

4. From Start, open Command Prompt, then Windows PowerShell (not Windows
   Terminal). Each should open in a Network Toolkit window. Type a command,
   such as echo hello or Get-Date, and press Enter. Then type exit.

5. Tell Claude what happened:
   - Did Command Prompt open in Network Toolkit? Did typing work?
   - Did Windows PowerShell open in Network Toolkit? Did typing work?
   - Anything odd (window title, resizing, closing, a second window)?

6. Undo everything:
     powershell -ExecutionPolicy Bypass -File .\Remove-NetworkToolkitTerminalTest.ps1
   This selects your previous default terminal again, prints a short test log
   (copy it to Claude if something did not work), removes the package and stops
   trusting the certificate.

The test certificate expires 30 days after the package was built; install it
before then.
