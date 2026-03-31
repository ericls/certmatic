import { useEffect, useState } from "react";
import { DomainSetup } from "./pages/DomainSetup";
import { ThemeToggle } from "./components/ThemeToggle";
import { sanitizeUrl } from "./utils/sanitizeUrl";

type Theme = "light" | "dark" | "system";

interface BackButton {
  url: string;
  text: string;
}

const sourceURL = "https://github.com/ericls/certmatic";

function applyTheme(theme: Theme) {
  const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  const isDark = theme === "dark" || (theme === "system" && prefersDark);
  document.documentElement.classList.toggle("dark", isDark);
}

export default function App() {
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem("certmatic-theme") as Theme) ?? "system",
  );
  const [backButton, setBackButton] = useState<BackButton | null>(null);

  useEffect(() => {
    applyTheme(theme);
    localStorage.setItem("certmatic-theme", theme);
  }, [theme]);

  useEffect(() => {
    if (theme !== "system") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => applyTheme("system");
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, [theme]);

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <nav className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        <div className="max-w-3xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            {backButton && (
              <a
                href={sanitizeUrl(backButton.url)}
                className="text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 flex items-center gap-1"
              >
                <span aria-hidden>←</span>
                {backButton.text}
              </a>
            )}
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
              Domain Setup
            </span>
          </div>
          <ThemeToggle theme={theme} onChange={setTheme} />
        </div>
      </nav>
      <main>
        <DomainSetup onBackButton={setBackButton} />
      </main>
      <footer className="border-t border-gray-200 bg-white/80 dark:border-gray-800 dark:bg-gray-900/80">
        <div className="max-w-3xl mx-auto px-4 py-4 text-xs text-gray-500 dark:text-gray-400 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <p>Certmatic Portal</p>
          <div className="flex items-center gap-4">
            <a
              href={sourceURL}
              target="_blank"
              rel="noreferrer"
              className="hover:text-gray-700 dark:hover:text-gray-200 underline underline-offset-2"
            >
              Source
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
