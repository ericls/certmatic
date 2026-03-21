import React from "react";

interface Props {
  n: number;
  title: string;
  children: React.ReactNode;
}

export function StepCard({ n, title, children }: Props) {
  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden">
      <div className="flex items-center gap-3 px-5 py-4 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        <span className="flex-shrink-0 w-7 h-7 rounded-full bg-primary-600 text-white text-sm font-bold flex items-center justify-center">
          {n}
        </span>
        <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100">{title}</h2>
      </div>
      <div className="px-5 py-5 bg-white dark:bg-gray-900">{children}</div>
    </div>
  );
}
