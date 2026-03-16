import { useEffect, useState } from "react";
import { DomainSetup } from "./pages/DomainSetup";
import { ThemeToggle } from "./components/ThemeToggle";

type Theme = "light" | "dark" | "system";

function applyTheme(theme: Theme) {
  const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  const isDark = theme === "dark" || (theme === "system" && prefersDark);
  document.documentElement.classList.toggle("dark", isDark);
}

export default function App() {
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem("certmatic-theme") as Theme) ?? "system",
  );

  useEffect(() => {
    applyTheme(theme);
    localStorage.setItem("certmatic-theme", theme);
  }, [theme]);

  // Re-apply when system preference changes (only matters in "system" mode).
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
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
            Domain Setup
          </span>
          <ThemeToggle theme={theme} onChange={setTheme} />
        </div>
      </nav>
      <main>
        <DomainSetup />
      </main>
    </div>
  );
}
