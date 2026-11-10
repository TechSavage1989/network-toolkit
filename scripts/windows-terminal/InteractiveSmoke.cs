using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;

// Synthetic acceptance fixture. The parent script limits it to a disposable
// Windows VM and verifies the installed executable path before using its HWND.
public static class DefaultTerminalSmoke
{
    private const uint CreateNewConsole = 0x10;
    private const uint KeyboardInput = 1;
    private const uint UnicodeKey = 4;
    private const uint KeyUp = 2;
    private const uint MouseLeftDown = 2;
    private const uint MouseLeftUp = 4;
    private const string HostClass = "NetworkToolkit.WebView2.Window";
    private const uint SaferScopeUser = 2;
    private const uint SaferLevelNormalUser = 0x20000;
    private const uint SaferLevelOpen = 1;
    private const int TokenIntegrityLevelClass = 25;
    private const uint GroupIntegrity = 0x20;

    public static uint LaunchNewConsole(string executable, string arguments, string workingDirectory)
    {
        RequireHostedRunner();
        if (!Path.IsPathRooted(executable) || !File.Exists(executable))
            throw new ArgumentException("Use the full path to a Windows console fixture.", "executable");

        var startup = new StartupInfo { Size = (uint)Marshal.SizeOf(typeof(StartupInfo)) };
        var command = new StringBuilder("\"" + executable + "\" " + arguments);
        ProcessInfo info;
        // Windows hands only unelevated consoles to another terminal. The hosted
        // runner account has no split token, so start the shell the way an
        // ordinary user's shell runs: Safer "normal user" token (Administrators
        // deny-only, privileges removed) at medium integrity.
        IntPtr level, token = IntPtr.Zero, sid = IntPtr.Zero, label = IntPtr.Zero;
        if (!SaferCreateLevel(SaferScopeUser, SaferLevelNormalUser, SaferLevelOpen, out level, IntPtr.Zero))
            throw new Win32Exception(Marshal.GetLastWin32Error());
        try
        {
            if (!SaferComputeTokenFromLevel(level, IntPtr.Zero, out token, 0, IntPtr.Zero))
                throw new Win32Exception(Marshal.GetLastWin32Error());
            if (!ConvertStringSidToSid("S-1-16-8192", out sid))
                throw new Win32Exception(Marshal.GetLastWin32Error());
            int size = Marshal.SizeOf(typeof(SidAndAttributes)) + GetLengthSid(sid);
            label = Marshal.AllocHGlobal(size);
            Marshal.StructureToPtr(new SidAndAttributes { Sid = sid, Attributes = GroupIntegrity }, label, false);
            if (!SetTokenInformation(token, TokenIntegrityLevelClass, label, (uint)size))
                throw new Win32Exception(Marshal.GetLastWin32Error());
            if (!CreateProcessAsUser(token, executable, command, IntPtr.Zero, IntPtr.Zero, false,
                                     CreateNewConsole, IntPtr.Zero, workingDirectory, ref startup, out info))
            {
                int error = Marshal.GetLastWin32Error();
                if (error != 1314 || !CreateProcessWithTokenW(token, 0, executable, command,
                        CreateNewConsole, IntPtr.Zero, workingDirectory, ref startup, out info))
                    throw new Win32Exception(error == 1314 ? Marshal.GetLastWin32Error() : error);
            }
        }
        finally
        {
            if (label != IntPtr.Zero) Marshal.FreeHGlobal(label);
            if (sid != IntPtr.Zero) LocalFree(sid);
            if (token != IntPtr.Zero) CloseHandle(token);
            SaferCloseLevel(level);
        }
        CloseHandle(info.Thread);
        CloseHandle(info.Process);
        return info.ProcessId;
    }

    // Mandatory label RID of a process (8192 medium, 12288 high), or -1.
    public static int ProcessIntegrity(uint processId)
    {
        IntPtr process = OpenProcess(0x1000, false, processId), token = IntPtr.Zero, buffer = IntPtr.Zero;
        if (process == IntPtr.Zero) return -1;
        try
        {
            if (!OpenProcessToken(process, 8, out token)) return -1;
            uint needed;
            GetTokenInformation(token, TokenIntegrityLevelClass, IntPtr.Zero, 0, out needed);
            if (needed == 0) return -1;
            buffer = Marshal.AllocHGlobal((int)needed);
            if (!GetTokenInformation(token, TokenIntegrityLevelClass, buffer, needed, out needed)) return -1;
            IntPtr labelSid = Marshal.ReadIntPtr(buffer);
            int count = Marshal.ReadByte(GetSidSubAuthorityCount(labelSid));
            return count == 0 ? -1 : Marshal.ReadInt32(GetSidSubAuthority(labelSid, (uint)(count - 1)));
        }
        finally
        {
            if (buffer != IntPtr.Zero) Marshal.FreeHGlobal(buffer);
            if (token != IntPtr.Zero) CloseHandle(token);
            CloseHandle(process);
        }
    }

    public static IntPtr FindWindow(uint expectedProcess)
    {
        RequireHostedRunner();
        IntPtr found = IntPtr.Zero;
        EnumWindows(delegate (IntPtr window, IntPtr ignored)
        {
            if (!IsHostWindow(window, expectedProcess))
                return true;
            found = window;
            return false;
        }, IntPtr.Zero);
        return found;
    }

    public static int ProbeActivation(string classId)
    {
        RequireHostedRunner();
        Guid clsid = new Guid(classId);
        Guid unknown = new Guid("00000000-0000-0000-C000-000000000046");
        int apartment = CoInitializeEx(IntPtr.Zero, 2);
        if (apartment < 0 && apartment != unchecked((int)0x80010106))
            return apartment;
        try
        {
            IntPtr instance;
            int result = CoCreateInstance(ref clsid, IntPtr.Zero, 4, ref unknown, out instance);
            if (instance != IntPtr.Zero)
                Marshal.Release(instance);
            return result;
        }
        finally
        {
            if (apartment >= 0)
                CoUninitialize();
        }
    }

    public static bool DesktopHasInput()
    {
        RequireHostedRunner();
        IntPtr desktop = GetThreadDesktop(GetCurrentThreadId());
        int receivesInput;
        uint bytesNeeded;
        return desktop != IntPtr.Zero &&
               GetUserObjectInformation(desktop, 6, out receivesInput, 4, out bytesNeeded) &&
               receivesInput != 0;
    }

    public static void SendCommand(IntPtr window, uint expectedProcess, string command)
    {
        RequireHostedRunner();
        if (!IsHostWindow(window, expectedProcess))
            throw new InvalidOperationException("The foreground target is not the expected Network Toolkit window.");
        ShowWindow(window, 9); // SW_RESTORE
        SetForegroundWindow(window);
        if (GetForegroundWindow() != window)
            throw new InvalidOperationException("Windows did not focus the expected Network Toolkit window.");

        Rect bounds;
        if (!GetClientRect(window, out bounds))
            throw new Win32Exception(Marshal.GetLastWin32Error());
        var point = new Point { X = bounds.Right * 2 / 3, Y = bounds.Bottom / 2 };
        if (!ClientToScreen(window, ref point) || !SetCursorPos(point.X, point.Y))
            throw new Win32Exception(Marshal.GetLastWin32Error());

        Send(window, new[] {
            new Input { Value = new InputValue { Mouse = new MouseEvent { Flags = MouseLeftDown } } },
            new Input { Value = new InputValue { Mouse = new MouseEvent { Flags = MouseLeftUp } } }
        });
        Thread.Sleep(100); // Let WebView finish processing the focus click.
        var keys = new Input[2 * (command.Length + 1)];
        for (int i = 0; i < command.Length; ++i)
        {
            keys[2 * i] = Key(0, command[i], UnicodeKey);
            keys[2 * i + 1] = Key(0, command[i], UnicodeKey | KeyUp);
        }
        keys[keys.Length - 2] = Key(0x0d, '\0', 0);
        keys[keys.Length - 1] = Key(0x0d, '\0', KeyUp);
        Send(window, keys);
    }

    private static void Send(IntPtr window, Input[] input)
    {
        if (GetForegroundWindow() != window)
            throw new InvalidOperationException("Focus moved away from the Network Toolkit fixture.");
        if (SendInput((uint)input.Length, input, Marshal.SizeOf(typeof(Input))) != input.Length)
            throw new Win32Exception(Marshal.GetLastWin32Error());
    }

    private static Input Key(ushort virtualKey, char codeUnit, uint flags)
    {
        return new Input
        {
            Kind = KeyboardInput,
            Value = new InputValue { Key = new KeyEvent { VirtualKey = virtualKey, Scan = codeUnit, Flags = flags } }
        };
    }

    private static bool IsHostWindow(IntPtr window, uint expectedProcess)
    {
        uint process;
        GetWindowThreadProcessId(window, out process);
        var name = new StringBuilder(128);
        return process == expectedProcess && IsWindowVisible(window) &&
               GetClassName(window, name, name.Capacity) > 0 && name.ToString() == HostClass;
    }

    private static void RequireHostedRunner()
    {
        if (Environment.GetEnvironmentVariable("GITHUB_ACTIONS") != "true" ||
            Environment.GetEnvironmentVariable("RUNNER_ENVIRONMENT") != "github-hosted")
            throw new InvalidOperationException("Use the interactive terminal fixture only on a disposable GitHub-hosted runner.");
    }

    private delegate bool WindowCallback(IntPtr window, IntPtr parameter);

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CreateProcess(string application, StringBuilder commandLine,
        IntPtr processAttributes, IntPtr threadAttributes, bool inherit, uint flags, IntPtr environment,
        string currentDirectory, ref StartupInfo startup, out ProcessInfo process);
    [DllImport("kernel32.dll")] private static extern bool CloseHandle(IntPtr handle);
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CreateProcessAsUser(IntPtr token, string application, StringBuilder commandLine,
        IntPtr processAttributes, IntPtr threadAttributes, bool inherit, uint flags, IntPtr environment,
        string currentDirectory, ref StartupInfo startup, out ProcessInfo process);
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CreateProcessWithTokenW(IntPtr token, uint logonFlags, string application, StringBuilder commandLine,
        uint flags, IntPtr environment, string currentDirectory, ref StartupInfo startup, out ProcessInfo process);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool SaferCreateLevel(uint scope, uint level, uint flags, out IntPtr handle, IntPtr reserved);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool SaferComputeTokenFromLevel(IntPtr level, IntPtr inToken, out IntPtr outToken, uint flags, IntPtr reserved);
    [DllImport("advapi32.dll")] private static extern bool SaferCloseLevel(IntPtr level);
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)] private static extern bool ConvertStringSidToSid(string sid, out IntPtr value);
    [DllImport("advapi32.dll")] private static extern int GetLengthSid(IntPtr sid);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool SetTokenInformation(IntPtr token, int kind, IntPtr information, uint length);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool GetTokenInformation(IntPtr token, int kind, IntPtr information, uint length, out uint needed);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool OpenProcessToken(IntPtr process, uint access, out IntPtr token);
    [DllImport("advapi32.dll")] private static extern IntPtr GetSidSubAuthorityCount(IntPtr sid);
    [DllImport("advapi32.dll")] private static extern IntPtr GetSidSubAuthority(IntPtr sid, uint index);
    [DllImport("kernel32.dll", SetLastError = true)] private static extern IntPtr OpenProcess(uint access, bool inherit, uint processId);
    [DllImport("kernel32.dll")] private static extern IntPtr LocalFree(IntPtr memory);
    [DllImport("kernel32.dll")] private static extern uint GetCurrentThreadId();
    [DllImport("ole32.dll")] private static extern int CoInitializeEx(IntPtr reserved, uint flags);
    [DllImport("ole32.dll")] private static extern void CoUninitialize();
    [DllImport("ole32.dll")] private static extern int CoCreateInstance(ref Guid clsid, IntPtr outer, uint context, ref Guid iid, out IntPtr instance);
    [DllImport("user32.dll")] private static extern bool EnumWindows(WindowCallback callback, IntPtr parameter);
    [DllImport("user32.dll")] private static extern IntPtr GetThreadDesktop(uint thread);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] private static extern bool GetUserObjectInformation(IntPtr handle, int index, out int value, uint size, out uint bytesNeeded);
    [DllImport("user32.dll")] private static extern bool IsWindowVisible(IntPtr window);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr window, out uint process);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] private static extern int GetClassName(IntPtr window, StringBuilder name, int count);
    [DllImport("user32.dll")] private static extern bool ShowWindow(IntPtr window, int command);
    [DllImport("user32.dll")] private static extern bool SetForegroundWindow(IntPtr window);
    [DllImport("user32.dll")] private static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll", SetLastError = true)] private static extern bool GetClientRect(IntPtr window, out Rect rect);
    [DllImport("user32.dll", SetLastError = true)] private static extern bool ClientToScreen(IntPtr window, ref Point point);
    [DllImport("user32.dll", SetLastError = true)] private static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll", SetLastError = true)] private static extern uint SendInput(uint count, Input[] input, int size);

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private struct StartupInfo
    {
        public uint Size;
        public string Reserved, Desktop, Title;
        public uint X, Y, XSize, YSize, XChars, YChars, FillAttribute, Flags;
        public ushort ShowWindow, ReservedSize;
        public IntPtr ReservedData, StdInput, StdOutput, StdError;
    }
    [StructLayout(LayoutKind.Sequential)]
    private struct ProcessInfo
    {
        public IntPtr Process, Thread;
        public uint ProcessId, ThreadId;
    }
    [StructLayout(LayoutKind.Sequential)]
    private struct SidAndAttributes { public IntPtr Sid; public uint Attributes; }
    [StructLayout(LayoutKind.Sequential)]
    private struct Rect { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential)]
    private struct Point { public int X, Y; }
    [StructLayout(LayoutKind.Sequential)]
    private struct Input { public uint Kind; public InputValue Value; }
    [StructLayout(LayoutKind.Explicit)]
    private struct InputValue
    {
        [FieldOffset(0)] public MouseEvent Mouse;
        [FieldOffset(0)] public KeyEvent Key;
    }
    [StructLayout(LayoutKind.Sequential)]
    private struct MouseEvent
    {
        public int X, Y;
        public uint Data, Flags, Time;
        public UIntPtr Extra;
    }
    [StructLayout(LayoutKind.Sequential)]
    private struct KeyEvent
    {
        public ushort VirtualKey, Scan;
        public uint Flags, Time;
        public UIntPtr Extra;
    }
}
