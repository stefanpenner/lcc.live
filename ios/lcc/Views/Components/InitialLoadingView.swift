import SwiftUI

struct InitialLoadingView: View {
    @State private var isAnimating = false
    @State private var phase: CGFloat = 0
    
    var body: some View {
        VStack(spacing: 24) {
            ZStack {
                // Outer pulsing ring - Liquid Glass
                Circle()
                    .stroke(
                        LinearGradient(
                            colors: [
                                Color.accentColor.opacity(0.3),
                                Color.accentColor.opacity(0.05)
                            ],
                            startPoint: .topLeading,
                            endPoint: .bottomTrailing
                        ),
                        lineWidth: 3
                    )
                    .frame(width: 80, height: 80)
                    .scaleEffect(isAnimating ? 1.2 : 1.0)
                    .opacity(isAnimating ? 0.0 : 1.0)
                
                // Middle ring
                Circle()
                    .stroke(
                        LinearGradient(
                            colors: [
                                Color.accentColor.opacity(0.5),
                                Color.accentColor.opacity(0.2)
                            ],
                            startPoint: .topLeading,
                            endPoint: .bottomTrailing
                        ),
                        lineWidth: 4
                    )
                    .frame(width: 60, height: 60)
                    .scaleEffect(isAnimating ? 1.15 : 1.0)
                    .opacity(isAnimating ? 0.2 : 1.0)
                
                // Inner core - Solid glass
                ZStack {
                    Circle()
                        .fill(.thinMaterial)
                        .frame(width: 50, height: 50)
                    
                    Circle()
                        .fill(
                            RadialGradient(
                                colors: [
                                    Color.accentColor.opacity(0.6),
                                    Color.accentColor.opacity(0.3),
                                    Color.accentColor.opacity(0.1)
                                ],
                                center: .center,
                                startRadius: 0,
                                endRadius: 25
                            )
                        )
                        .frame(width: 50, height: 50)
                    
                    // Spinning shimmer
                    Circle()
                        .trim(from: 0, to: 0.3)
                        .stroke(
                            Color.accentColor.opacity(0.8),
                            style: StrokeStyle(lineWidth: 3, lineCap: .round)
                        )
                        .frame(width: 50, height: 50)
                        .rotationEffect(.degrees(phase * 360))
                }
                .shadow(color: Color.accentColor.opacity(0.4), radius: 10)
            }
            
            VStack(spacing: 8) {
                Text("Loading Streams")
                    .font(.headline)
                    .foregroundStyle(.primary)
                
                Text("Preparing your live feed...")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
        }
        .onAppear {
            // Pulsing animation
            withAnimation(
                .easeInOut(duration: 1.5)
                .repeatForever(autoreverses: true)
            ) {
                isAnimating = true
            }
            
            // Spinning animation
            withAnimation(
                .linear(duration: 2.0)
                .repeatForever(autoreverses: false)
            ) {
                phase = 1.0
            }
        }
    }
}

