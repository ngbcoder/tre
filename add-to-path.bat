@echo off
REM ---------------------------------------------------------------------------
REM  Adds the folder containing tre.exe to the CURRENT USER's PATH.
REM  Keeps this .bat file in the same folder as tre.exe and double-click it.
REM ---------------------------------------------------------------------------
setlocal EnableDelayedExpansion

set "EXE=%~dp0tre.exe"
set "EXEDIR=%~dp0"
if "%EXEDIR:~-1%"=="\" set "EXEDIR=%EXEDIR:~0,-1%"

if not exist "%EXE%" (
    echo ERROR: tre.exe not found in:
    echo   %EXEDIR%
    echo.
    echo Keep this .bat file next to tre.exe and run it again.
    pause
    exit /b 1
)

echo Target folder: %EXEDIR%
echo.

REM --- Read the CURRENT USER path (not the combined process path) -----------
set "USERPATH="
for /f "tokens=2,*" %%A in ('reg query "HKCU\Environment" /v Path 2^>nul') do set "USERPATH=%%B"

REM --- Already there? -------------------------------------------------------
set "FOUND="
for %%E in ("!USERPATH:;=" " ") do (
    set "CUR=%%~E"
    if "!CUR:~-1!"=="\" set "CUR=!CUR:~0,-1%"
    if /i "!CUR!"=="!EXEDIR!" set "FOUND=1"
)

if defined FOUND (
    echo Already in your user PATH - nothing to do.
    goto :verify
)

REM --- Build the new value --------------------------------------------------
if "!USERPATH!"=="" (
    set "NEWPATH=%EXEDIR%"
) else (
    set "NEWPATH=!USERPATH!;!EXEDIR!"
)

REM --- Write it (reg add avoids setx's 1024 character limit) ---------------
reg add "HKCU\Environment" /v Path /t REG_EXPAND_SZ /d "!NEWPATH!" /f >nul
if errorlevel 1 (
    echo FAILED to write the registry. Try running this as Administrator.
    pause
    exit /b 1
)

echo Added to your user PATH.

:verify
echo.
echo Checking the copy of PATH in this window...
set "CURRENT=%PATH%"
for %%E in ("!CURRENT:;=" " ") do (
    set "CUR=%%~E"
    if "!CUR:~-1!"=="\" set "CUR=!CUR:~0,-1%"
    if /i "!CUR!"=="!EXEDIR!" set "LIVE=1"
)

if defined LIVE (
    echo   already active in this window - "tre" should work now.
) else (
    echo   NOT active yet.
    echo   Close and open a NEW terminal window, then run: tre --help
)

echo.
echo To remove it later, delete that folder from:
echo   System Properties ^> Environment Variables ^> User variables ^> Path
pause