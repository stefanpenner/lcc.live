import SwiftUI

struct ShimmerView: View {
    let width: CGFloat
    let height: CGFloat
    let colorScheme: ColorScheme

    @State private var pulsing = false

    var body: some View {
        RoundedRectangle(cornerRadius: 4)
            .fill(Color.secondary)
            .opacity(pulsing ? 0.6 : 0.3)
            .frame(width: width, height: height)
            .onAppear {
                withAnimation(
                    .easeInOut(duration: 1)
                    .repeatForever(autoreverses: true)
                ) {
                    pulsing = true
                }
            }
            .accessibilityLabel("Loading image")
    }
}
