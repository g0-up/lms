import { useQueryClient } from "@tanstack/react-query";
import { LogOut, Menu, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, Outlet, useLocation, useNavigate } from "react-router";
import { loginUrl, resetSession, useLogout, useMe } from "@/features/auth";
import { homeOf, ROLE_VI, type Role } from "@/shared/domain";
import { useMediaQuery } from "@/shared/hooks/use-media-query";
import { BrandMark } from "@/shared/layout/brand-mark";
import { isNavCurrent, NAV } from "@/shared/layout/nav";
import { cn } from "@/shared/lib/cn";
import { initials } from "@/shared/lib/format";
import { IconButton } from "@/shared/ui/icon-button";
import { Sheet, SheetClose, SheetContent, SheetTitle, SheetTrigger } from "@/shared/ui/sheet";

const MOBILE_QUERY = "(max-width: 720px)";

const topbarIconClass = "text-white hover:bg-white/12 hover:text-white";

function NavLinks({ role, pathname, onNavigate, variant }: {
  role: Role;
  pathname: string;
  onNavigate?: () => void;
  variant: "bar" | "drawer";
}) {
  return NAV[role].map((item) => {
    const current = isNavCurrent(role, item.to, pathname);
    return (
      <Link
        key={item.to}
        to={item.to}
        aria-current={current ? "page" : undefined}
        onClick={onNavigate}
        className={cn(
          "font-ui text-[14px] leading-5 font-bold tracking-[0.5px] whitespace-nowrap text-white uppercase no-underline",
          "transition-colors duration-[180ms] ease-brand hover:text-surface-blue-100",
          variant === "bar"
            ? "inline-block border-b-3 border-transparent px-3.5 py-[18px] aria-[current=page]:border-accent-orange max-[960px]:px-2.5"
            : "block border-l-3 border-transparent px-4 py-3.5 aria-[current=page]:border-accent-orange",
        )}
      >
        {item.label}
      </Link>
    );
  });
}

/**
 * Signed-in frame: skip link, navy topbar (brand, role nav, account, logout) and the page in
 * `main#main`. At ≤720px the nav moves into a full-width drawer opened from the menu button.
 */
export function AppShell() {
  const { data: user } = useMe();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const logout = useLogout();
  const isMobile = useMediaQuery(MOBILE_QUERY);
  const [navOpen, setNavOpen] = useState(false);

  // The session can end while the shell is open (the background /auth/me refetch answers 401):
  // leave like any other expired session, unless the user is the one logging out.
  const sessionEnded = user === null && logout.isIdle;
  const returnTo = location.pathname + location.search;
  useEffect(() => {
    if (!sessionEnded) return;
    resetSession(queryClient);
    void navigate(loginUrl(returnTo), { replace: true });
  }, [sessionEnded, queryClient, navigate, returnTo]);

  if (!user) return null;

  const home = homeOf(user.role);
  const closeNav = () => {
    setNavOpen(false);
  };

  return (
    <>
      <a
        href="#main"
        className="absolute -top-25 left-3 z-100 inline-flex min-h-11 items-center rounded-sm bg-navy-900 px-3 py-2 font-semibold text-white no-underline focus:top-3 focus:outline-2 focus:outline-offset-2 focus:outline-accent-crimson"
      >
        Bỏ qua điều hướng
      </a>
      <header className="bg-navy-700 text-white">
        <div className="mx-auto flex min-h-14 max-w-page items-center gap-2 px-6 max-[720px]:gap-1 max-[720px]:px-4">
          <Sheet open={navOpen && isMobile} onOpenChange={setNavOpen}>
            <SheetTrigger asChild>
              <IconButton aria-label="Mở menu" className={cn("hidden max-[720px]:inline-grid", topbarIconClass)}>
                <Menu aria-hidden="true" />
              </IconButton>
            </SheetTrigger>
            <SheetContent
              side="left"
              showClose={false}
              aria-describedby={undefined}
              className="bg-navy-700 text-white"
            >
              <div className="flex min-h-14 items-center gap-1 px-4">
                <SheetClose asChild>
                  <IconButton aria-label="Đóng" className={topbarIconClass}>
                    <X aria-hidden="true" />
                  </IconButton>
                </SheetClose>
                <SheetTitle className="flex items-center gap-3 text-lg text-white">
                  <BrandMark />
                  <span className="font-display font-bold tracking-[-0.3px]">GoUp LMS</span>
                </SheetTitle>
              </div>
              <nav aria-label="Chính" className="flex flex-col py-2">
                <NavLinks role={user.role} pathname={location.pathname} onNavigate={closeNav} variant="drawer" />
              </nav>
            </SheetContent>
          </Sheet>

          <Link
            to={home}
            aria-label="GoUp LMS"
            className="mr-4 flex min-h-11 min-w-11 flex-none items-center justify-center gap-3 text-white no-underline max-[720px]:mr-0"
          >
            <BrandMark />
            <span className="font-display text-lg font-bold tracking-[-0.3px] max-[720px]:hidden">
              GoUp
              <span className="ml-1.5 font-sans text-[14px] font-normal tracking-[0.5px] opacity-80">LMS</span>
            </span>
          </Link>

          <nav id="nav" aria-label="Chính" className="flex items-center gap-0.5 max-[720px]:hidden">
            <NavLinks role={user.role} pathname={location.pathname} variant="bar" />
          </nav>

          <div className="ml-auto flex min-w-0 items-center gap-2">
            <div className="flex flex-none items-center gap-2">
              <span
                aria-hidden="true"
                className="inline-grid size-8 flex-none place-items-center rounded-full bg-surface-blue-100 text-xs font-semibold tracking-normal text-navy-900"
              >
                {initials(user.name)}
              </span>
              <span className="text-sm leading-[1.2] whitespace-nowrap max-[960px]:sr-only">
                {user.name}
                <small className="block text-xs opacity-75">{ROLE_VI[user.role]}</small>
              </span>
            </div>
            <IconButton
              aria-label="Đăng xuất"
              title="Đăng xuất"
              className={topbarIconClass}
              disabled={logout.isPending}
              onClick={() => {
                logout.mutate();
              }}
            >
              <LogOut aria-hidden="true" />
            </IconButton>
          </div>
        </div>
      </header>
      <main
        id="main"
        tabIndex={-1}
        className="mx-auto max-w-page px-6 pt-8 pb-16 focus:outline-none max-[720px]:px-4 max-[720px]:pt-6"
      >
        <Outlet />
      </main>
    </>
  );
}
