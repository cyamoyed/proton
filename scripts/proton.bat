@echo off
REM Wrapper script for proton
REM Ensures proton.exe is run from the same directory as this script.

"%~dp0proton.exe" %*
