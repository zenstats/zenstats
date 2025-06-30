export default function checkLogin() {
  const authInfoStr = localStorage.getItem('authInfo');
  if (!authInfoStr) return false;

  try {
    const authInfo = JSON.parse(authInfoStr);
    return authInfo !== null && authInfo.expire * 1000 > new Date().getTime();
  } catch {
    return false;
  }
}