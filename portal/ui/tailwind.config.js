/** @type {import('tailwindcss').Config} */
import colors from "tailwindcss/colors";

export default {
  darkMode: "class",
  content: ["./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        primary: colors.blue,
      },
    },
  },
  plugins: [],
};
