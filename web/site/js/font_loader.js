document.addEventListener("DOMContentLoaded", () => {
    const body = document.body;
    const loader = document.getElementById("fontLoader");
    const progress = document.getElementById("fontProgress");
    const progressText = document.getElementById("fontProgressText");

    if (!body || !loader || !progress || !progressText) {
        return;
    }

    if (!document.fonts || !document.fonts.load) {
        body.classList.remove("font-loading");
        return;
    }

    let isFinished = false;
    let progressValue = 8;

    function setProgress(value) {
        const clamped = Math.max(0, Math.min(100, value));
        progress.style.width = `${clamped}%`;
        progressText.textContent = `${clamped}%`;
    }

    function finishLoading() {
        if (isFinished) {
            return;
        }
        isFinished = true;
        setProgress(100);
        body.classList.remove("font-loading");
    }

    const tick = window.setInterval(() => {
        if (isFinished) {
            window.clearInterval(tick);
            return;
        }
        progressValue = Math.min(progressValue + Math.random() * 9 + 4, 92);
        setProgress(Math.floor(progressValue));
    }, 140);

    const fontPromises = [
        document.fonts.load('1em "WDXLLubrifontJPN"')
    ];

    Promise.all(fontPromises)
        .then(() => {
            finishLoading();
        })
        .catch(() => {
            finishLoading();
        });

});
