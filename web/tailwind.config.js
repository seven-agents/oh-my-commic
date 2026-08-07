/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Warm storybook palette — Studio Ghibli inspired
        cream: '#FBF7EF',
        parchment: '#F6EEDD',
        ink: '#4A3F35', // warm dark-brown text, never pure black
        'ink-soft': '#7A6E60',
        sky: '#8EC5E8',
        'sky-deep': '#6FB0DA',
        meadow: '#A8C97F',
        'meadow-deep': '#8FB565',
        peach: '#F2A17C',
        coral: '#E8846B',
        sun: '#F5C86B',
        cloud: '#FFFFFF',
      },
      fontFamily: {
        display: ['"Baloo 2"', '"PingFang SC"', '"Microsoft YaHei"', 'system-ui', 'sans-serif'],
        body: ['Quicksand', '"PingFang SC"', '"Microsoft YaHei"', 'system-ui', 'sans-serif'],
      },
      borderRadius: {
        '4xl': '2rem',
        '5xl': '2.5rem',
      },
      boxShadow: {
        soft: '0 8px 24px -8px rgba(74, 63, 53, 0.18)',
        'soft-lg': '0 16px 40px -12px rgba(74, 63, 53, 0.22)',
        'soft-sm': '0 4px 12px -4px rgba(74, 63, 53, 0.16)',
      },
      keyframes: {
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-12px)' },
        },
        'float-slow': {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-20px)' },
        },
        twinkle: {
          '0%, 100%': { opacity: '0.3', transform: 'scale(0.9)' },
          '50%': { opacity: '1', transform: 'scale(1.1)' },
        },
        'pop-in': {
          '0%': { opacity: '0', transform: 'scale(0.94) translateY(8px)' },
          '100%': { opacity: '1', transform: 'scale(1) translateY(0)' },
        },
      },
      animation: {
        float: 'float 6s ease-in-out infinite',
        'float-slow': 'float-slow 9s ease-in-out infinite',
        twinkle: 'twinkle 2.4s ease-in-out infinite',
        'pop-in': 'pop-in 0.4s ease-out both',
      },
    },
  },
  plugins: [],
}
