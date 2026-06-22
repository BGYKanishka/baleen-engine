import { Component, type ErrorInfo, type ReactNode } from 'react';
interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
}

class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
    errorInfo: null
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error, errorInfo: null };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("Uncaught error:", error, errorInfo);
    this.setState({ errorInfo });
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col h-screen items-center justify-center bg-white dark:bg-gray-900 text-gray-900 dark:text-white p-8">
          <div className="bg-red-50 dark:bg-red-900/30 border border-red-500 rounded-lg p-6 max-w-3xl w-full">
            <h1 className="text-xl font-bold text-red-600 dark:text-red-500 mb-4">UI Crashed!</h1>
            <div className="bg-white dark:bg-black p-4 rounded overflow-auto text-sm font-mono text-red-800 dark:text-red-300 border border-red-200 dark:border-transparent">
              <p className="font-bold mb-2">{this.state.error && this.state.error.toString()}</p>
              <pre>{this.state.errorInfo?.componentStack}</pre>
            </div>
            <p className="mt-4 text-gray-600 dark:text-gray-400 text-sm">
              Copy this error and paste it in the chat!
            </p>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;