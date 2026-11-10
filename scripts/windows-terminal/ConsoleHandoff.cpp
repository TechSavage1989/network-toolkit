#include <windows.h>
#include <appmodel.h>
#include <objbase.h>

#include <array>
#include <atomic>
#include <cstdint>
#include <cwchar>
#ifdef NTK_HANDOFF_EXPERIMENT_DIAGNOSTICS
#include <cstdio>
#include <cstring>
#endif
#include <new>
#include <string>
#include <vector>

namespace {

// This IID and pointer ABI match the Microsoft OpenConsoleProxy registration.
// https://github.com/microsoft/terminal/blob/main/src/host/proxy/ITerminalHandoff.idl
struct TerminalStartupInfo;
struct __declspec(uuid("6F23DA90-15C5-4203-9DB0-64E73F1B1B00")) ITerminalHandoff3 : IUnknown {
    virtual HRESULT STDMETHODCALLTYPE EstablishPtyHandoff(
        HANDLE* input, HANDLE* output, HANDLE signal, HANDLE reference,
        HANDLE server, HANDLE client, const TerminalStartupInfo* startup) = 0;
};

constexpr CLSID terminalClass{0xb60e0829, 0x5bf3, 0x4a4a, {0xac, 0xb9, 0xc6, 0x61, 0xd8, 0x5b, 0x72, 0x82}};
constexpr wchar_t openConsolePackage[] = L"Microsoft.WindowsTerminal_8wekyb3d8bbwe";
constexpr DWORD pipeBuffer = 128 * 1024;
constexpr DWORD adoptionTimeout = 20000;
HANDLE sessionCreated;
HANDLE sessionReleased;

class Handle {
public:
    explicit Handle(HANDLE handle = nullptr) noexcept : value(handle) {}
    ~Handle() { if (*this) CloseHandle(value); }
    Handle(const Handle&) = delete;
    Handle& operator=(const Handle&) = delete;

    HANDLE get() const noexcept { return value; }
    explicit operator bool() const noexcept { return value && value != INVALID_HANDLE_VALUE; }
    HANDLE release() noexcept {
        const auto result = value;
        value = nullptr;
        return result;
    }
    HANDLE* put() noexcept { return &value; }

private:
    HANDLE value;
};

struct Identity {
    std::vector<BYTE> user;
    DWORD session{};
    DWORD integrity{};

    PSID sid() const noexcept {
        return reinterpret_cast<const TOKEN_USER*>(user.data())->User.Sid;
    }
};

bool tokenInfo(HANDLE token, TOKEN_INFORMATION_CLASS kind, std::vector<BYTE>& info) {
    DWORD size = 0;
    GetTokenInformation(token, kind, nullptr, 0, &size);
    if (!size || GetLastError() != ERROR_INSUFFICIENT_BUFFER) return false;
    info.resize(size);
    return GetTokenInformation(token, kind, info.data(), size, &size) != FALSE;
}

bool processIdentity(HANDLE process, Identity& identity) {
    Handle token;
    DWORD size = 0;
    if (!OpenProcessToken(process, TOKEN_QUERY, token.put()) ||
        !GetTokenInformation(token.get(), TokenSessionId, &identity.session, sizeof(identity.session), &size) ||
        !tokenInfo(token.get(), TokenUser, identity.user)) {
        return false;
    }
    std::vector<BYTE> integrity;
    if (!tokenInfo(token.get(), TokenIntegrityLevel, integrity)) return false;
    const auto sid = reinterpret_cast<const TOKEN_MANDATORY_LABEL*>(integrity.data())->Label.Sid;
    if (!IsValidSid(sid) || !IsValidSid(identity.sid())) return false;
    const auto count = *GetSidSubAuthorityCount(sid);
    if (!count) return false;
    identity.integrity = *GetSidSubAuthority(sid, count - 1);
    return true;
}

HRESULT lastError() noexcept {
    const DWORD error = GetLastError();
    return error ? HRESULT_FROM_WIN32(error) : E_FAIL;
}

#ifdef NTK_HANDOFF_EXPERIMENT_DIAGNOSTICS
void writeDiagnostic(const char* stage, HRESULT status) {
    constexpr wchar_t familyPrefix[] = L"NetworkToolkit.DefaultTerminalExperiment_";
    std::array<wchar_t, PACKAGE_FAMILY_NAME_MAX_LENGTH + 1> family{};
    UINT32 familySize = static_cast<UINT32>(family.size());
    if (GetCurrentPackageFamilyName(&familySize, family.data()) != ERROR_SUCCESS ||
        std::wcsncmp(family.data(), familyPrefix, ARRAYSIZE(familyPrefix) - 1) != 0) return;

    Handle token;
    std::vector<BYTE> user;
    if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, token.put()) ||
        !tokenInfo(token.get(), TokenUser, user)) return;
    wchar_t account[64]{};
    wchar_t domain[128]{};
    DWORD accountSize = ARRAYSIZE(account);
    DWORD domainSize = ARRAYSIZE(domain);
    SID_NAME_USE accountType{};
    const auto sid = reinterpret_cast<const TOKEN_USER*>(user.data())->User.Sid;
    // Diagnostics are compiled only into the CI experiment package and are
    // written only for the hosted runner account that owns the session.
    if (!LookupAccountSidW(nullptr, sid, account, &accountSize, domain, &domainSize, &accountType) ||
        accountType != SidTypeUser || _wcsicmp(account, L"runneradmin") != 0) return;

    // COM can inherit a different process environment from the test launcher.
    // Use the real token's profile and the fixture's own package LocalState.
    std::wstring directory(32768, L'\0');
    DWORD length = static_cast<DWORD>(directory.size());
    const auto userenv = LoadLibraryExW(L"userenv.dll", nullptr, LOAD_LIBRARY_SEARCH_SYSTEM32);
    if (!userenv) return;
    using ProfileDirectory = BOOL (WINAPI*)(HANDLE, LPWSTR, LPDWORD);
    const auto address = GetProcAddress(userenv, "GetUserProfileDirectoryW");
    ProfileDirectory profile = nullptr;
    static_assert(sizeof(profile) == sizeof(address));
    std::memcpy(&profile, &address, sizeof(profile));
    const auto resolved = profile && profile(token.get(), directory.data(), &length);
    FreeLibrary(userenv);
    if (!resolved) return;
    directory.resize(std::wcslen(directory.c_str()));
    directory += L"\\AppData\\Local\\Packages";
    CreateDirectoryW(directory.c_str(), nullptr);
    directory += L"\\";
    directory += family.data();
    CreateDirectoryW(directory.c_str(), nullptr);
    directory += L"\\LocalState";
    CreateDirectoryW(directory.c_str(), nullptr);
    directory += L"\\broker.log";

    const Handle log(CreateFileW(directory.c_str(), FILE_APPEND_DATA | FILE_READ_ATTRIBUTES,
                                 FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
                                 nullptr, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, nullptr));
    LARGE_INTEGER size{};
    if (!log || !GetFileSizeEx(log.get(), &size)) return;
    char line[96]{};
    const auto written = std::snprintf(line, sizeof(line), "%s 0x%08lX\r\n", stage,
                                       static_cast<unsigned long>(status));
    if (written <= 0 || static_cast<size_t>(written) >= sizeof(line) || size.QuadPart + written > 64 * 1024) return;
    DWORD count = 0;
    WriteFile(log.get(), line, static_cast<DWORD>(written), &count, nullptr);
}

void diagnostic(const char* stage, HRESULT status = S_OK) noexcept {
    const auto error = GetLastError();
    try { writeDiagnostic(stage, status); } catch (...) {}
    SetLastError(error);
}
#else
void diagnostic(const char*, HRESULT = S_OK) noexcept {}
#endif

HRESULT reported(const char* stage, HRESULT status) noexcept {
    diagnostic(stage, status);
    return status;
}

bool matchesIdentity(const Identity& receiver, HANDLE process) {
    Identity sender;
    if (!processIdentity(process, sender)) {
        diagnostic("identity-token", lastError());
        return false;
    }
    if (sender.session != receiver.session) {
        diagnostic("identity-session", E_ACCESSDENIED);
        return false;
    }
    if (sender.integrity != receiver.integrity) {
        diagnostic("identity-integrity", E_ACCESSDENIED);
        return false;
    }
    if (!EqualSid(sender.sid(), receiver.sid())) {
        diagnostic("identity-user", E_ACCESSDENIED);
        return false;
    }
    return true;
}

bool isOpenConsole(HANDLE server) {
    std::array<wchar_t, PACKAGE_FAMILY_NAME_MAX_LENGTH + 1> family{};
    UINT32 size = static_cast<UINT32>(family.size());
    const auto familyStatus = GetPackageFamilyName(server, &size, family.data());
    if (familyStatus != ERROR_SUCCESS) {
        diagnostic("source-family-query", HRESULT_FROM_WIN32(familyStatus));
        return false;
    }
    if (_wcsicmp(family.data(), openConsolePackage) != 0) {
        diagnostic("source-family-mismatch", E_ACCESSDENIED);
        return false;
    }
    std::wstring image(32768, L'\0');
    DWORD length = static_cast<DWORD>(image.size());
    if (!QueryFullProcessImageNameW(server, 0, image.data(), &length)) {
        diagnostic("source-image-query", lastError());
        return false;
    }
    image.resize(length);
    const auto separator = image.find_last_of(L'\\');
    if (separator == std::wstring::npos || _wcsicmp(image.c_str() + separator + 1, L"OpenConsole.exe") != 0) {
        diagnostic("source-image-mismatch", E_ACCESSDENIED);
        return false;
    }
    return true;
}

HRESULT createVtPipe(bool appReads, Handle& app, Handle& console) {
    GUID id{};
    const auto result = CoCreateGuid(&id);
    if (FAILED(result)) return result;
    wchar_t idText[39];
    if (!StringFromGUID2(id, idText, ARRAYSIZE(idText))) return E_FAIL;
    const std::wstring name = std::wstring{L"\\\\.\\pipe\\NetworkToolkit.Terminal."} + idText;
    const auto direction = appReads ? PIPE_ACCESS_INBOUND : PIPE_ACCESS_OUTBOUND;
    *app.put() = CreateNamedPipeW(name.c_str(), direction | FILE_FLAG_FIRST_PIPE_INSTANCE | FILE_FLAG_OVERLAPPED,
                                 PIPE_TYPE_BYTE | PIPE_WAIT | PIPE_REJECT_REMOTE_CLIENTS,
                                 1, pipeBuffer, pipeBuffer, 0, nullptr);
    if (!app) return lastError();

    Handle connected(CreateEventW(nullptr, TRUE, FALSE, nullptr));
    if (!connected) return lastError();
    OVERLAPPED operation{};
    operation.hEvent = connected.get();
    if (!ConnectNamedPipe(app.get(), &operation)) {
        const auto error = GetLastError();
        if (error != ERROR_IO_PENDING && error != ERROR_PIPE_CONNECTED) return HRESULT_FROM_WIN32(error);
    }
    *console.put() = CreateFileW(name.c_str(), appReads ? GENERIC_WRITE : GENERIC_READ,
                                 0, nullptr, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
    DWORD count = 0;
    if (!console) {
        const auto error = lastError();
        CancelIoEx(app.get(), &operation);
        GetOverlappedResult(app.get(), &operation, &count, TRUE);
        return error;
    }
    if (!GetOverlappedResult(app.get(), &operation, &count, TRUE)) return lastError();
    ULONG clientProcess = 0;
    if (!GetNamedPipeClientProcessId(app.get(), &clientProcess)) return lastError();
    return clientProcess == GetCurrentProcessId() ? S_OK : E_ACCESSDENIED;
}

HRESULT launchTerminal(const std::array<HANDLE, 5>& handles, HANDLE ready) {
    std::wstring folder(32768, L'\0');
    const auto length = GetModuleFileNameW(nullptr, folder.data(), static_cast<DWORD>(folder.size()));
    if (!length || length >= folder.size()) return reported("renderer-resolve", lastError());
    folder.resize(length);
    const auto separator = folder.find_last_of(L'\\');
    if (separator == std::wstring::npos) return reported("renderer-resolve", E_FAIL);
    folder.resize(separator);
    const std::wstring executable = folder + L"\\Network_Toolkit.exe";

    std::array<Handle, 5> inherited;
    std::array<HANDLE, 5> allowlist{};
    std::wstring arguments = L"\"" + executable + L"\" --terminal-handoff";
    const auto self = GetCurrentProcess();
    for (size_t i = 0; i < handles.size(); ++i) {
        if (!DuplicateHandle(self, handles[i], self, inherited[i].put(), 0, TRUE, DUPLICATE_SAME_ACCESS)) {
            return reported("renderer-inherit", lastError());
        }
        allowlist[i] = inherited[i].get();
        arguments += L" " + std::to_wstring(reinterpret_cast<uintptr_t>(allowlist[i]));
    }

    SIZE_T size = 0;
    InitializeProcThreadAttributeList(nullptr, 1, 0, &size);
    if (!size) return reported("renderer-attribute-init", lastError());
    std::vector<BYTE> memory(size);
    const auto attributes = reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(memory.data());
    if (!InitializeProcThreadAttributeList(attributes, 1, 0, &size)) {
        return reported("renderer-attribute-init", lastError());
    }

    STARTUPINFOEXW startup{};
    startup.StartupInfo.cb = sizeof(startup);
    startup.lpAttributeList = attributes;
    PROCESS_INFORMATION process{};
    HRESULT result = S_OK;
    if (!UpdateProcThreadAttribute(attributes, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST, allowlist.data(),
                                   sizeof(allowlist), nullptr, nullptr)) {
        result = reported("renderer-attribute-update", lastError());
    } else if (!CreateProcessW(executable.c_str(), arguments.data(), nullptr, nullptr, TRUE, EXTENDED_STARTUPINFO_PRESENT,
                               nullptr, folder.c_str(), &startup.StartupInfo, &process)) {
        result = reported("renderer-create", lastError());
    }
    DeleteProcThreadAttributeList(attributes);
    if (FAILED(result)) return result;

    const Handle child(process.hProcess);
    const Handle childThread(process.hThread);
    const HANDLE waits[] = {child.get(), ready};
    diagnostic("renderer-created");
    switch (WaitForMultipleObjects(ARRAYSIZE(waits), waits, FALSE, adoptionTimeout)) {
    case WAIT_OBJECT_0 + 1: return reported("renderer-ready", S_OK);
    case WAIT_OBJECT_0: return reported("renderer-exit-early", HRESULT_FROM_WIN32(ERROR_PROCESS_ABORTED));
    case WAIT_TIMEOUT: result = reported("renderer-timeout", HRESULT_FROM_WIN32(ERROR_TIMEOUT)); break;
    default: result = reported("renderer-wait", lastError()); break;
    }
    TerminateProcess(child.get(), ERROR_CANCELLED);
    return result;
}

void releaseServer() noexcept {
    if (CoReleaseServerProcess() == 0) SetEvent(sessionReleased);
}

class Handoff final : public ITerminalHandoff3 {
public:
    Handoff() noexcept { CoAddRefServerProcess(); }

    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void** result) override {
        if (!result) return E_POINTER;
        *result = nullptr;
        if (iid != __uuidof(IUnknown) && iid != __uuidof(ITerminalHandoff3)) return E_NOINTERFACE;
        *result = static_cast<ITerminalHandoff3*>(this);
        AddRef();
        return S_OK;
    }

    ULONG STDMETHODCALLTYPE AddRef() override { return ++refs; }
    ULONG STDMETHODCALLTYPE Release() override {
        const auto count = --refs;
        if (!count) delete this;
        return count;
    }

    HRESULT STDMETHODCALLTYPE EstablishPtyHandoff(
        HANDLE* input, HANDLE* output, HANDLE signal, HANDLE reference,
        HANDLE server, HANDLE client, const TerminalStartupInfo* startup) override try {
        diagnostic("establish-enter");
        if (!input || !output) return reported("handoff-outputs", E_POINTER);
        *input = nullptr;
        *output = nullptr;
        DWORD referenceFlags = 0;
        if (!startup) return reported("handoff-startup-info", E_INVALIDARG);
        if (GetFileType(signal) != FILE_TYPE_PIPE) return reported("handoff-signal", E_INVALIDARG);
        if (!GetHandleInformation(reference, &referenceFlags)) {
            diagnostic("handoff-reference", lastError());
            return E_INVALIDARG;
        }
        Identity receiver;
        if (!isOpenConsole(server)) return reported("source-check", E_ACCESSDENIED);
        diagnostic("source-verified");
        if (!processIdentity(GetCurrentProcess(), receiver) || receiver.session == 0) {
            return reported("receiver-identity", E_ACCESSDENIED);
        }
        if (!matchesIdentity(receiver, server)) return reported("source-identity", E_ACCESSDENIED);
        if (!matchesIdentity(receiver, client)) return reported("client-identity", E_ACCESSDENIED);
        diagnostic("identity-verified");
        if (handedOff.exchange(true)) return reported("handoff-already-used", HRESULT_FROM_WIN32(ERROR_ALREADY_INITIALIZED));

        Handle appRead, consoleWrite, appWrite, consoleRead;
        auto result = createVtPipe(true, appRead, consoleWrite);
        if (FAILED(result)) return reported("vt-read-pipe", result);
        result = createVtPipe(false, appWrite, consoleRead);
        if (FAILED(result)) return reported("vt-write-pipe", result);
        Handle ready(CreateEventW(nullptr, TRUE, FALSE, nullptr));
        if (!ready) return reported("ready-event", lastError());

        result = launchTerminal({appRead.get(), appWrite.get(), signal, reference, ready.get()}, ready.get());
        if (FAILED(result)) return result;
        // The console host reads terminal input and writes terminal output.
        *input = consoleRead.release();
        *output = consoleWrite.release();
        return reported("handoff-accepted", S_OK);
    } catch (const std::bad_alloc&) {
        return reported("handoff-exception", E_OUTOFMEMORY);
    } catch (...) {
        return reported("handoff-exception", E_FAIL);
    }

private:
    ~Handoff() { releaseServer(); }
    std::atomic<ULONG> refs{1};
    std::atomic<bool> handedOff{false};
};

class Factory final : public IClassFactory {
public:
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void** result) override {
        if (!result) return E_POINTER;
        *result = nullptr;
        if (iid != __uuidof(IUnknown) && iid != __uuidof(IClassFactory)) return E_NOINTERFACE;
        *result = static_cast<IClassFactory*>(this);
        AddRef();
        return S_OK;
    }

    ULONG STDMETHODCALLTYPE AddRef() override { return ++refs; }
    ULONG STDMETHODCALLTYPE Release() override {
        const auto count = --refs;
        if (!count) delete this;
        return count;
    }

    HRESULT STDMETHODCALLTYPE CreateInstance(IUnknown* outer, REFIID iid, void** result) override {
        diagnostic("class-request");
        if (!result) return E_POINTER;
        *result = nullptr;
        if (outer) return CLASS_E_NOAGGREGATION;
        SetEvent(sessionCreated);
        auto handoff = new (std::nothrow) Handoff;
        if (!handoff) {
            SetEvent(sessionReleased);
            return reported("class-instance", E_OUTOFMEMORY);
        }
        const auto status = handoff->QueryInterface(iid, result);
        diagnostic("class-instance", status);
        handoff->Release();
        return status;
    }

    HRESULT STDMETHODCALLTYPE LockServer(BOOL lock) override {
        if (lock) CoAddRefServerProcess();
        else releaseServer();
        return S_OK;
    }

private:
    std::atomic<ULONG> refs{1};
};

} // namespace

int WINAPI wWinMain(HINSTANCE, HINSTANCE, PWSTR commandLine, int) {
    diagnostic("entry");
    if (_wcsicmp(commandLine, L"-Embedding") != 0 && _wcsicmp(commandLine, L"/Embedding") != 0) {
        diagnostic("embedding-argument", HRESULT_FROM_WIN32(ERROR_BAD_ARGUMENTS));
        return ERROR_BAD_ARGUMENTS;
    }
    Handle created(CreateEventW(nullptr, TRUE, FALSE, nullptr));
    Handle released(CreateEventW(nullptr, TRUE, FALSE, nullptr));
    if (!created || !released) {
        diagnostic("startup-events", lastError());
        return 1;
    }
    sessionCreated = created.get();
    sessionReleased = released.get();

    const auto init = CoInitializeEx(nullptr, COINIT_MULTITHREADED);
    diagnostic("com-init", init);
    if (FAILED(init)) return 1;
    auto factory = new (std::nothrow) Factory;
    if (!factory) {
        diagnostic("class-factory", E_OUTOFMEMORY);
        CoUninitialize();
        return 1;
    }
    DWORD registration = 0;
    const auto result = CoRegisterClassObject(terminalClass, factory, CLSCTX_LOCAL_SERVER, REGCLS_SINGLEUSE, &registration);
    diagnostic("class-register", result);
    factory->Release();
    if (SUCCEEDED(result)) {
        const auto creation = WaitForSingleObject(created.get(), 20000);
        if (creation == WAIT_OBJECT_0) {
            WaitForSingleObject(released.get(), INFINITE);
        } else {
            diagnostic("class-wait", creation == WAIT_TIMEOUT ? HRESULT_FROM_WIN32(ERROR_TIMEOUT) : lastError());
        }
        diagnostic("class-revoke", CoRevokeClassObject(registration));
    }
    CoUninitialize();
    return FAILED(result) ? 1 : 0;
}
