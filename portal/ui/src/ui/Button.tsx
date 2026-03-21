import React from "react";

type ButtonVariant = "primary" | "outline" | "ghost";
type ButtonSize = "md" | "sm";

const variantClass: Record<ButtonVariant, string> = {
  primary:
    "bg-primary-600 text-white hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed",
  outline:
    "border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed",
  ghost: "text-primary-600 dark:text-primary-400 hover:underline",
};

const sizeClass: Record<ButtonSize, string> = {
  md: "px-4 py-2 text-sm font-medium",
  sm: "px-3 py-1 text-xs font-medium",
};

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

export function Button({
  variant = "primary",
  size = "md",
  className = "",
  ...props
}: ButtonProps) {
  const padding = variant === "ghost" ? "" : sizeClass[size];
  return (
    <button
      className={`rounded-lg inline-flex items-center justify-center ${padding} ${variantClass[variant]} ${className}`.trim()}
      {...props}
    />
  );
}
