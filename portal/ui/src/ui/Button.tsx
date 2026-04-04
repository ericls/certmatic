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

export function ExternalLinkIcon({ className = "" }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      <path d="M15 3h6v6" />
      <path d="M10 14 21 3" />
      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
    </svg>
  );
}

interface LinkButtonProps extends React.AnchorHTMLAttributes<HTMLAnchorElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

export function LinkButton({
  variant = "primary",
  size = "md",
  className = "",
  ...props
}: LinkButtonProps) {
  const padding = variant === "ghost" ? "" : sizeClass[size];
  return (
    <a
      className={`rounded-lg inline-flex items-center justify-center ${padding} ${variantClass[variant]} ${className}`.trim()}
      {...props}
    />
  );
}
