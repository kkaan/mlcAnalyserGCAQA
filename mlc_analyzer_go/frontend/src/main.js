import {EventsOn, Dialog} from '../../wailsjs/runtime'; // Ensure this path is correct

document.addEventListener('DOMContentLoaded', () => {
    const splashScreen = document.getElementById('splashScreen');
    const mainAppContainer = document.getElementById('app');
    const statusLog = document.getElementById('statusLog'); // Already defined below, ensure one def

    // Splash screen logic
    setTimeout(() => {
        if (splashScreen) splashScreen.style.display = 'none';
        if (mainAppContainer) mainAppContainer.classList.remove('initially-hidden');
    }, 2500); // Display splash for 2.5 seconds

    // Existing JS logic (ensure variables are scoped correctly or re-declared if needed)
    const csvPathInput = document.getElementById('csvPath');
    const browseCsvBtn = document.getElementById('browseCsvBtn');
    const pdfPathInput = document.getElementById('pdfPath');
    const browsePdfBtn = document.getElementById('browsePdfBtn');
    const toleranceInput = document.getElementById('tolerance');
    const generateBtn = document.getElementById('generateBtn');
    // const statusLog = document.getElementById('statusLog'); // Already got this above

    function logMessage(message) {
        const time = new Date().toLocaleTimeString();
        if (statusLog) { // Check if statusLog exists
            statusLog.textContent += `[${time}] ${message}\n`;
            statusLog.scrollTop = statusLog.scrollHeight;
        } else {
            console.log(`Status log element not found. Message: ${message}`);
        }
    }

    // Ensure EventsOn is only called once per event type if this script is re-executed/concatenated
    // For simplicity, assuming fresh execution here.
    EventsOn('statusUpdate', (message) => {
        logMessage(message);
    });
    EventsOn('clearLog', () => {
        if(statusLog) statusLog.textContent = '';
    });

    EventsOn('generationStart', () => {
        if(generateBtn) {
            generateBtn.disabled = true;
            generateBtn.textContent = 'Generating...';
        }
    });

    EventsOn('generationComplete', (success, message) => {
        if(generateBtn) {
            generateBtn.disabled = false;
            generateBtn.textContent = 'Generate Report';
        }
        logMessage(message);
    });


    if(browseCsvBtn) {
        browseCsvBtn.addEventListener('click', async () => {
            try {
                const result = await Dialog({ // Using Wails built-in Dialog
                    Title: 'Select Input CSV File',
                    Filters: [{DisplayName: 'CSV Files (*.csv)', Pattern: '*.csv'}],
                });
                if (result && csvPathInput) {
                    csvPathInput.value = result;
                    if (result.toLowerCase().endsWith('.csv') && pdfPathInput) {
                        pdfPathInput.value = result.substring(0, result.length - 4) + '_Report_Go.pdf';
                    }
                }
            } catch (e) {
                logMessage('Error selecting CSV file: ' + e);
            }
        });
    }

    if(browsePdfBtn) {
        browsePdfBtn.addEventListener('click', async () => {
            try {
                const result = await Dialog({ // Using Wails built-in Dialog
                    Title: 'Save PDF Report As',
                    DefaultFilename: pdfPathInput ? pdfPathInput.value : 'MLC_Analysis_Report_Go.pdf',
                    Filters: [{DisplayName: 'PDF Files (*.pdf)', Pattern: '*.pdf'}],
                    CanCreateDirectories: true,
                });
                if (result && pdfPathInput) {
                    pdfPathInput.value = result;
                }
            } catch (e) {
                logMessage('Error selecting PDF save path: ' + e);
            }
        });
    }

    if(generateBtn) {
        generateBtn.addEventListener('click', async () => {
            const csvFile = csvPathInput ? csvPathInput.value : '';
            const pdfFile = pdfPathInput ? pdfPathInput.value : '';
            const toleranceStr = toleranceInput ? toleranceInput.value : '1.0';

            if (!csvFile || !pdfFile) {
                logMessage('Error: Please select input CSV and output PDF file paths.');
                return;
            }
            let toleranceNum = parseFloat(toleranceStr);
            if (isNaN(toleranceNum)) {
                logMessage('Error: Invalid tolerance value. Please enter a number.');
                return;
            }

            try {
                // Wails v2 call to Go
                await window.go.main.App.HandleGenerateReport(csvFile, pdfFile, toleranceNum);
            } catch (e) {
                logMessage('Error calling Go to generate report: ' + e);
                if(generateBtn) { // Ensure button is re-enabled
                    generateBtn.disabled = false;
                    generateBtn.textContent = 'Generate Report';
                }
            }
        });
    }

    logMessage('Frontend initialized. Ready.'); // This will now appear after splash
});
