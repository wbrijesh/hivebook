// Internal brand-logo set.
//
// These are the official, full-color vector marks for third-party identity
// providers, copied verbatim from their source SVGs (Google and Microsoft
// from Wikimedia Commons; Atlassian from the official mark). They are
// intentionally NOT styleable: a brand logo must always render in its
// correct colors and proportions, so the only thing a caller may change is
// the rendered size. Do not add `className`, `color`, or `fill` props.

type BrandLogoProps = {
  /** Rendered width and height, in pixels. */
  size?: number
}

const DEFAULT_SIZE = 20

export function GoogleLogo({ size = DEFAULT_SIZE }: BrandLogoProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
      focusable="false"
    >
      <path
        fill="#4285F4"
        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
      />
      <path
        fill="#34A853"
        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
      />
      <path
        fill="#FBBC05"
        d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
      />
      <path
        fill="#EA4335"
        d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
      />
    </svg>
  )
}

export function MicrosoftLogo({ size = DEFAULT_SIZE }: BrandLogoProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 23 23"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
      focusable="false"
    >
      <path fill="#f35325" d="M1 1h10v10H1z" />
      <path fill="#81bc06" d="M12 1h10v10H12z" />
      <path fill="#05a6f0" d="M1 12h10v10H1z" />
      <path fill="#ffba08" d="M12 12h10v10H12z" />
    </svg>
  )
}

export function AtlassianLogo({ size = DEFAULT_SIZE }: BrandLogoProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 256 256"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
      focusable="false"
    >
      <defs>
        <linearGradient
          id="atlassian-logo-gradient"
          x1="99.6865531%"
          y1="15.8007988%"
          x2="39.8359011%"
          y2="97.4378355%"
        >
          <stop stopColor="#0052CC" offset="0%" />
          <stop stopColor="#2684FF" offset="92.3%" />
        </linearGradient>
      </defs>
      <path
        fill="url(#atlassian-logo-gradient)"
        d="M75.7929022 117.949352c-3.8194672-4.080172-9.7708279-3.848901-12.366664 1.342771L.791180865 244.565041c-1.161181079 2.321166-1.03750329 5.07811 .326852385 7.285979 1.36435565 2.207869 3.77477065 3.551721 6.37018037 3.551496H94.716435c2.8552051.06619 5.480675-1.556915 6.698034-4.140442C120.223468 212.37359 108.82814 153.245434 75.7929022 117.949352Z"
      />
      <path
        fill="#2681FF"
        d="M121.756071 4.0114918C86.7234975 59.5164098 89.0348008 120.989508 112.109989 167.141287l42.060394 84.120787c1.26832 2.536659 3.861966 4.139021 6.697033 4.139041h87.226819c2.59541.000224 5.005825-1.343628 6.370181-3.551497 1.364355-2.207868 1.488033-4.964813.326852-7.286005 0 0-117.346648-234.72693393-120.2985-240.59980285C131.853481-1.29371311 125.14944-1.36519672 121.756071 4.0114918Z"
      />
    </svg>
  )
}
