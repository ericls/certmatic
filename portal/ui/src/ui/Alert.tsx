import React from "react";

type AlertVariant = "success" | "warning" | "error" | "neutral";

const variantClass: Record<AlertVariant, string> = {
  success:
    "bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800 text-green-800 dark:text-green-300",
  warning:
    "bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-800 text-yellow-800 dark:text-yellow-300",
  error:
    "bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800 text-red-800 dark:text-red-300",
  neutral:
    "bg-gray-50 dark:bg-gray-800 border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-400",
};

const defaultIcon: Record<AlertVariant, string> = {
  success: "✓",
  warning: "⏳",
  error: "✗",
  neutral: "⏳",
};

interface AlertProps {
  variant: AlertVariant;
  icon?: string;
  children: React.ReactNode;
}

export function Alert({ variant, icon, children }: AlertProps) {
  return (
    <div className={`flex items-center gap-2 p-3 border rounded-lg ${variantClass[variant]}`}>
      <span className="text-lg">{icon ?? defaultIcon[variant]}</span>
      <span className="text-sm font-medium">{children}</span>
    </div>
  );
}
