type Theme = "light" | "dark" | "system";

interface Props {
  theme: Theme;
  onChange: (theme: Theme) => void;
}

const options: { value: Theme; label: string }[] = [
  { value: "system", label: "System" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

export function ThemeToggle({ theme, onChange }: Props) {
  return (
    <select
      value={theme}
      onChange={(e) => onChange(e.target.value as Theme)}
      className="text-xs border border-gray-200 dark:border-gray-600 rounded-md px-2 py-1 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 cursor-pointer"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}
